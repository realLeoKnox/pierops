package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/access"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
)

func operationRouter(t *testing.T) *gin.Engine {
	t.Helper()
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:pierops_router_test?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	for _, table := range []any{&models.AccessGrant{}, &models.OperationAudit{}, &models.Session{}, &models.User{}, &models.Client{}, &models.Task{}} {
		if err := db.Where("1 = 1").Delete(table).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{access.Owner, access.Operator} {
		if err := db.Create(&models.User{UUID: role, Username: role, Passwd: "test-only", AccessRole: role}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Session{UUID: role, Session: "test-session-" + role, Expires: time.Now().UTC().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range []string{"node-a", "node-b"} {
		if err := db.Create(&models.Client{UUID: node, Token: "test-only-token-" + node, Name: node, Remark: "test-only-private-note"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.AccessGrant{UserUUID: access.Operator, ClientUUID: "node-a", Action: access.NodeRead}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AccessGrant{UserUUID: access.Operator, ClientUUID: "node-a", Action: access.FileRead}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.IdentityMiddleware())
	Register(r)
	return r
}

func operatorRequest(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "test-session-operator"})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPierOpsRESTAndRPCScope(t *testing.T) {
	r := operationRouter(t)
	for _, p := range []struct{ method, path string }{
		{"GET", "/api/admin/client/node-b"}, {"GET", "/api/admin/client/node-a/token"},
		{"GET", "/api/admin/client/node-b/terminal"}, {"POST", "/api/admin/client/node-a/file/upload"},
		{"GET", "/api/admin/client/node-b/file/download?path=/test-only"}, {"GET", "/api/admin/settings/"},
		{"GET", "/api/admin/access/users"}, {"GET", "/api/admin/task/all"},
	} {
		if w := operatorRequest(r, p.method, p.path, nil); w.Code != http.StatusForbidden {
			t.Errorf("%s %s: status=%d", p.method, p.path, w.Code)
		}
	}
	w := operatorRequest(r, "GET", "/api/admin/client/node-a", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "test-only-token") || strings.Contains(w.Body.String(), "private-note") {
		t.Fatal("Scoped node response exposed privileged metadata")
	}
	for _, p := range []any{
		map[string]any{"uuid": "node-b", "path": "/test-only"}, []any{"node-b", "/test-only"},
	} {
		w := operatorRequest(r, "POST", "/api/rpc2", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:fileStat", "params": p})
		var response rpc.JsonRpcResponse
		if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Error == nil || response.Error.Code != rpc.PermissionDenied {
			t.Fatal("RPC bypassed node scope")
		}
	}
	w = operatorRequest(r, "POST", "/api/rpc2", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "common:getNodes"})
	if strings.Contains(w.Body.String(), "test-only-token") || strings.Contains(w.Body.String(), "private-note") {
		t.Fatal("Legacy public RPC exposed private node metadata")
	}
	w = operatorRequest(r, "POST", "/api/admin/task/exec", map[string]any{"command": "test-only-command", "clients": []string{"node-a", "node-b"}})
	if w.Code != 403 {
		t.Fatal("REST multi-node exec scope bypass")
	}
	var tasks int64
	dbcore.GetDBInstance().Model(&models.Task{}).Count(&tasks)
	if tasks != 0 {
		t.Fatal("Unauthorized task was created")
	}
	for _, path := range []string{"/api/clients/terminal", "/api/clients/v2/rpc", "/api/clients/transfer/test-only"} {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: "test-session-owner"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatal("User entered an Agent-only endpoint")
		}
	}
}

func TestPierOpsWebSocketRevalidatesSession(t *testing.T) {
	r := operationRouter(t)
	server := httptest.NewServer(r)
	defer server.Close()
	headers := http.Header{"Cookie": []string{"session_token=test-session-operator"}, "Origin": []string{server.URL}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	call := func(method string) *rpc.JsonRpcResponse {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: method}); err != nil {
			t.Fatal(err)
		}
		var response rpc.JsonRpcResponse
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		return &response
	}
	if call("admin:listClients").Error != nil {
		t.Fatal("Valid scoped session denied")
	}
	if err := dbcore.GetDBInstance().Model(&models.Session{}).Where("uuid = ?", access.Operator).Update("expires", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if response := call("admin:listClients"); response.Error == nil || response.Error.Code != rpc.PermissionDenied {
		t.Fatal("WebSocket cached an expired session")
	}
}

func TestPierOpsPreviewTokenRevocation(t *testing.T) {
	r := operationRouter(t)
	w := operatorRequest(r, "GET", "/api/admin/client/node-a/file/preview-token?path=/test-only", nil)
	var response struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Data.Token == "" {
		t.Fatal("Preview token not issued")
	}
	if err := dbcore.GetDBInstance().Where("user_uuid = ?", access.Operator).Delete(&models.AccessGrant{}).Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/preview/client/node-a/file/download?preview_token="+response.Data.Token, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("Preview token survived permission revocation")
	}
}
