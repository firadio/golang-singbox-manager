package handlers

import (
	"os"
	"os/exec"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/config"
	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// SettingsHandler 系统设置处理器
type SettingsHandler struct {
	config     *config.Config
	configPath string
	mtClient   *mikrotik.Client
}

// NewSettingsHandler 创建系统设置处理器
func NewSettingsHandler(cfg *config.Config, configPath string, mtClient *mikrotik.Client) *SettingsHandler {
	return &SettingsHandler{
		config:     cfg,
		configPath: configPath,
		mtClient:   mtClient,
	}
}

// GetSettings 获取系统设置
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	// 返回配置时隐藏密码
	settings := gin.H{
		"web_port":               h.config.Server.Port,
		"auth_enabled":           h.config.Auth.Enabled,
		"auth_password_set":      h.config.Auth.Password != "",
		"mikrotik_enabled":       h.config.Mikrotik.Enabled,
		"mikrotik_address":       h.config.Mikrotik.Address,
		"mikrotik_username":      h.config.Mikrotik.Username,
		"mikrotik_port":          h.config.Mikrotik.Port,
		"mikrotik_routing_table": h.config.Mikrotik.RoutingTable,
	}
	Success(c, settings)
}

// UpdateSettings 更新系统设置
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var updateData struct {
		WebPort               int    `json:"web_port"`
		AuthEnabled           bool   `json:"auth_enabled"`
		AuthPassword          string `json:"auth_password"`
		MikrotikEnabled       bool   `json:"mikrotik_enabled"`
		MikrotikAddress       string `json:"mikrotik_address"`
		MikrotikUsername      string `json:"mikrotik_username"`
		MikrotikPassword      string `json:"mikrotik_password"`
		MikrotikPort          int    `json:"mikrotik_port"`
		MikrotikRoutingTable  string `json:"mikrotik_routing_table"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		Error(c, 3001, "Invalid request body")
		return
	}

	// 记录旧端口
	oldPort := h.config.Server.Port

	// 更新配置
	h.config.Server.Port = updateData.WebPort
	h.config.Auth.Enabled = updateData.AuthEnabled
	if updateData.AuthPassword != "" {
		h.config.Auth.Password = updateData.AuthPassword
	}
	h.config.Mikrotik.Enabled = updateData.MikrotikEnabled
	h.config.Mikrotik.Address = updateData.MikrotikAddress
	h.config.Mikrotik.Username = updateData.MikrotikUsername
	if updateData.MikrotikPassword != "" {
		h.config.Mikrotik.Password = updateData.MikrotikPassword
	}
	h.config.Mikrotik.Port = updateData.MikrotikPort
	if updateData.MikrotikRoutingTable != "" {
		h.config.Mikrotik.RoutingTable = updateData.MikrotikRoutingTable
	}

	// 保存配置文件
	if err := config.SaveConfig(h.configPath, h.config); err != nil {
		log.Errorf("Failed to save config: %v", err)
		Error(c, 3002, "Failed to save settings")
		return
	}

	log.Info("System settings updated successfully")

	portChanged := oldPort != updateData.WebPort
	restartRequired := portChanged ||
		h.config.Auth.Enabled != updateData.AuthEnabled ||
		h.config.Mikrotik.Enabled != updateData.MikrotikEnabled

	Success(c, gin.H{
		"message":          "Settings updated successfully",
		"restart_required": restartRequired,
		"port_changed":     portChanged,
		"new_port":         updateData.WebPort,
	})
}

// TestMikrotikConnection 测试Mikrotik连接
func (h *SettingsHandler) TestMikrotikConnection(c *gin.Context) {
	var testData struct {
		Address  string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
		Port     int    `json:"port"`
	}

	if err := c.ShouldBindJSON(&testData); err != nil {
		Error(c, 3003, "Invalid request body")
		return
	}

	// 如果密码为空，使用配置文件中的密码
	password := testData.Password
	if password == "" {
		password = h.config.Mikrotik.Password
		log.Debug("Using saved password for Mikrotik connection test")
	}

	// 创建临时配置测试连接
	testConfig := &config.MikrotikConfig{
		Enabled:  true,
		Address:  testData.Address,
		Username: testData.Username,
		Password: password,
		Port:     testData.Port,
	}

	testClient := mikrotik.NewClient(testConfig)
	if err := testClient.TestConnection(); err != nil {
		log.Errorf("Mikrotik connection test failed: %v", err)
		Error(c, 3004, "Connection failed: "+err.Error())
		return
	}

	Success(c, gin.H{"message": "Connection successful"})
}

// Login 登录验证
func (h *SettingsHandler) Login(c *gin.Context) {
	var loginData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&loginData); err != nil {
		Error(c, 4001, "Invalid request body")
		return
	}

	if !h.config.Auth.Enabled {
		Error(c, 4002, "Authentication is not enabled")
		return
	}

	if loginData.Password != h.config.Auth.Password {
		Error(c, 4003, "Invalid password")
		return
	}

	// 返回 token（简化处理：token 就是密码）
	Success(c, gin.H{
		"token":   h.config.Auth.Password,
		"message": "Login successful",
	})
}

// RestartService 重启服务
func (h *SettingsHandler) RestartService(c *gin.Context) {
	log.Info("Restart service requested")

	// 先返回成功响应
	Success(c, gin.H{"message": "Service restart initiated"})

	// 延迟2秒后重启服务（给响应足够的时间返回）
	go func() {
		time.Sleep(2 * time.Second)
		log.Info("Restarting service via systemctl...")

		cmd := exec.Command("systemctl", "restart", "singbox-manager")
		if err := cmd.Run(); err != nil {
			log.Errorf("Failed to restart service: %v", err)
		} else {
			log.Info("Service restart command executed")
		}
	}()
}

// ShutdownService 停止服务
func (h *SettingsHandler) ShutdownService(c *gin.Context) {
	log.Info("Shutdown service requested")

	// 先返回成功响应
	Success(c, gin.H{"message": "Service shutdown initiated"})

	// 延迟1秒后退出
	go func() {
		time.Sleep(1 * time.Second)
		log.Info("Shutting down service...")
		os.Exit(0)
	}()
}
