package ginutil

import "github.com/gin-gonic/gin"

// PrivateNoStore prevents authenticated delivery responses (including errors)
// from being reused by browser or intermediary caches.
func PrivateNoStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Writer.Header().Add("Vary", "Authorization")
}
