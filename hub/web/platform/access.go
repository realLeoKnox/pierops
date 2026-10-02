package platform

import (
	_ "embed"
	"github.com/gin-gonic/gin"
	"net/http"
)

//go:embed access.html
var accessPage []byte

//go:embed access.js
var accessScript []byte

func AccessPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/html; charset=utf-8", accessPage)
}

func AccessScript(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", accessScript)
}
