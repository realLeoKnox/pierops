package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/access"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func OperationMeta(c *gin.Context) *rpc.ContextMeta {
	p := GetPrincipal(c)
	if p == nil {
		p = IdentifyPrincipal(c)
	}
	session, _ := c.Cookie("session_token")
	return &rpc.ContextMeta{Principal: p, SessionToken: session, APIKeyToken: strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")}
}

func AuthorizeOperation(c *gin.Context, action, node string) bool {
	if access.Default().Authorize(OperationMeta(c), action, node) != nil {
		RespondError(c, http.StatusForbidden, "Operation not permitted")
		return false
	}
	return true
}

// Administrative REST endpoints are owner-only except this exact route list.
// Each listed handler performs its own node/action check before doing any work.
func RequirePlatformAccess() gin.HandlerFunc {
	operations := map[string]bool{
		"GET /api/admin/client/:uuid/terminal":           true,
		"POST /api/admin/client/:uuid/file/upload":       true,
		"GET /api/admin/client/:uuid/file/download":      true,
		"HEAD /api/admin/client/:uuid/file/download":     true,
		"GET /api/admin/client/:uuid/file/preview-token": true,
		"POST /api/admin/task/exec":                      true,
		"GET /api/admin/task/:task_id/result/:uuid":      true,
		"GET /api/admin/client/list":                     true,
		"GET /api/admin/client/:uuid":                    true,
		"GET /api/admin/access/self":                     true,
	}
	return func(c *gin.Context) {
		if !operations[c.Request.Method+" "+c.FullPath()] && !AuthorizeOperation(c, access.Manage, "") {
			c.Abort()
			return
		}
		c.Next()
	}
}
