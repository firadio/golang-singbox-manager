package middleware

import (
	"github.com/firadio/golang-singbox-manager/internal/config"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果认证未启用，直接通过
		if !cfg.Auth.Enabled {
			c.Next()
			return
		}

		// 从 cookie 获取 session_token
		sessionToken, err := c.Cookie("session_token")
		if err != nil || sessionToken == "" {
			c.JSON(401, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			c.Abort()
			return
		}

		// 简单验证: session_token 就是密码
		if sessionToken != cfg.Auth.Password {
			c.JSON(401, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
