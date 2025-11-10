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

		// 从 Authorization header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			c.Abort()
			return
		}

		// 检查格式: "Bearer <token>"
		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			c.JSON(401, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			c.Abort()
			return
		}

		token := authHeader[len(prefix):]

		// 简单验证: token 就是密码
		if token != cfg.Auth.Password {
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
