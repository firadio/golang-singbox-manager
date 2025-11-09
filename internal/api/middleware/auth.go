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

		// 从header获取密码
		password := c.GetHeader("X-Auth-Password")
		if password == "" {
			// 尝试从query参数获取
			password = c.Query("password")
		}

		// 验证密码
		if password != cfg.Auth.Password {
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
