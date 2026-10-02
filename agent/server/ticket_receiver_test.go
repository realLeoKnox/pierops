package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	pkg_flags "github.com/komari-monitor/komari-agent/cmd/flags"
	"github.com/komari-monitor/komari-agent/internal/localpolicy"
	v2 "github.com/komari-monitor/komari-agent/protocol/v2"
)

func TestLocalReceiverRequiresTicketBeforeFileOperation(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	oldFS := fileFS
	f, err := localpolicy.NewFiles([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	fileFS = f
	defer func() { fileFS = oldFS; f.Close() }()
	oldConfig := *pkg_flags.GlobalConfig
	defer func() { *pkg_flags.GlobalConfig = oldConfig }()
	flags.NodeUUID = "receiver-node"
	flags.Token = "receiver-test-only-token-00000"
	flags.DisableWebSsh = false
	flags.EnableFileWrite = true
	flags.DisableCompression = true
	previousVerifier := operationVerifier
	operationVerifier = v2.NewTicketVerifier(time.Now().Add(-time.Second))
	defer func() { operationVerifier = previousVerifier }()
	results := make(chan v2.FileResult, 8)
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req v2.Request
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Error("invalid result request")
		}
		var result v2.FileResult
		if v2.BindParams(req.Params, &result) != nil {
			t.Error("invalid result")
		}
		results <- result
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","result":{"status":"success"}}`))
	}))
	defer sink.Close()
	flags.Endpoint = sink.URL
	receive := func() v2.FileResult {
		select {
		case result := <-results:
			return result
		case <-time.After(3 * time.Second):
			t.Fatal("file result missing")
			return v2.FileResult{}
		}
	}
	sign := func(id, path string) any {
		p, e := v2.SignOperation(flags.NodeUUID, operationVerifier.Epoch(), "owner", flags.Token, v2.MethodAgentFile, v2.FileOperation{UUID: flags.NodeUUID, RequestID: id, Op: "create", Args: map[string]any{"path": path}}, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(p)
		var wire any
		json.Unmarshal(raw, &wire)
		return wire
	}
	target := filepath.Join(root, "allowed.txt")
	signed := sign("allowed", target)
	if !processV2Event(nil, v2.MethodAgentFile, signed, "", nil) || !receive().OK {
		t.Fatal("valid operation rejected")
	}
	if e := os.WriteFile(target, []byte("preserved"), 0600); e != nil {
		t.Fatal(e)
	}
	processV2Event(nil, v2.MethodAgentFile, signed, "", nil)
	if content, e := os.ReadFile(target); e != nil || string(content) != "preserved" {
		t.Fatal("replayed create executed")
	}
	missing := filepath.Join(root, "unsigned.txt")
	processV2Event(nil, v2.MethodAgentFile, v2.FileOperation{UUID: flags.NodeUUID, RequestID: "unsigned", Op: "create", Args: map[string]any{"path": missing}}, "", nil)
	if receive().OK {
		t.Fatal("unsigned operation succeeded")
	}
	if _, e := os.Stat(missing); !os.IsNotExist(e) {
		t.Fatal("unsigned operation created file")
	}
	escape := filepath.Join(outside, "forbidden.txt")
	processV2Event(nil, v2.MethodAgentFile, sign("escape", escape), "", nil)
	if receive().OK {
		t.Fatal("signed outside path succeeded")
	}
	if _, e := os.Stat(escape); !os.IsNotExist(e) {
		t.Fatal("outside file created")
	}
	flags.EnableFileWrite = false
	processV2Event(nil, v2.MethodAgentFile, sign("disabled", filepath.Join(root, "disabled.txt")), "", nil)
	if receive().OK {
		t.Fatal("disabled local capability succeeded")
	}
}
