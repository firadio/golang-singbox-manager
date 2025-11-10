package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/cert"
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
		"device_name":            h.config.Server.DeviceName,
		"web_port":               h.config.Server.Port,
		"https_enable":           h.config.Server.HTTPSEnable,
		"https_port":             h.config.Server.HTTPSPort,
		"auto_cert":              h.config.Server.AutoCert,
		"cert_file":              h.config.Server.CertFile,
		"key_file":               h.config.Server.KeyFile,
		"auth_enabled":           h.config.Auth.Enabled,
		"auth_username":          h.config.Auth.Username,
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
		DeviceName            string `json:"device_name"`
		WebPort               int    `json:"web_port"`
		HTTPSEnable           bool   `json:"https_enable"`
		HTTPSPort             int    `json:"https_port"`
		AutoCert              bool   `json:"auto_cert"`
		CertContent           string `json:"cert_content"`
		KeyContent            string `json:"key_content"`
		AuthEnabled           bool   `json:"auth_enabled"`
		AuthUsername          string `json:"auth_username"`
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
	oldHTTPSEnable := h.config.Server.HTTPSEnable
	oldHTTPSPort := h.config.Server.HTTPSPort

	// 更新配置
	if updateData.DeviceName != "" {
		h.config.Server.DeviceName = updateData.DeviceName
	}
	h.config.Server.Port = updateData.WebPort
	h.config.Server.HTTPSEnable = updateData.HTTPSEnable
	if updateData.HTTPSPort > 0 {
		h.config.Server.HTTPSPort = updateData.HTTPSPort
	}
	h.config.Server.AutoCert = updateData.AutoCert

	// 处理证书
	if updateData.HTTPSEnable {
		// 转换为绝对路径
		certFile := h.config.Server.CertFile
		keyFile := h.config.Server.KeyFile
		if !filepath.IsAbs(certFile) {
			certFile = filepath.Join(filepath.Dir(h.configPath), certFile)
		}
		if !filepath.IsAbs(keyFile) {
			keyFile = filepath.Join(filepath.Dir(h.configPath), keyFile)
		}

		if updateData.AutoCert {
			// 自动生成证书
			if err := cert.GenerateSelfSignedCert(certFile, keyFile); err != nil {
				log.Errorf("Failed to generate certificate: %v", err)
				Error(c, 3010, "Failed to generate certificate")
				return
			}
			log.Info("Self-signed certificate generated successfully")
		} else if updateData.CertContent != "" && updateData.KeyContent != "" {
			// 手动保存证书
			if err := cert.SaveCertAndKey(updateData.CertContent, updateData.KeyContent, certFile, keyFile); err != nil {
				log.Errorf("Failed to save certificate: %v", err)
				Error(c, 3011, "Failed to save certificate")
				return
			}
			log.Info("Certificate and key saved successfully")
		}

		// 验证证书
		if err := cert.ValidateCertAndKey(certFile, keyFile); err != nil {
			log.Errorf("Certificate validation failed: %v", err)
			Error(c, 3012, "Certificate validation failed: "+err.Error())
			return
		}
	}
	h.config.Auth.Enabled = updateData.AuthEnabled
	h.config.Auth.Username = updateData.AuthUsername
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
	httpsChanged := oldHTTPSEnable != updateData.HTTPSEnable || oldHTTPSPort != updateData.HTTPSPort
	restartRequired := portChanged ||
		httpsChanged ||
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

	// 如果配置了用户名，需要验证用户名
	if h.config.Auth.Username != "" {
		if loginData.Username != h.config.Auth.Username {
			Error(c, 4004, "Invalid username")
			return
		}
	}

	// 验证密码
	if loginData.Password != h.config.Auth.Password {
		Error(c, 4003, "Invalid password")
		return
	}

	// 设置 cookie（使用密码作为 session_token）
	c.SetSameSite(2) // SameSiteLaxMode = 2
	c.SetCookie(
		"session_token",        // name
		h.config.Auth.Password, // value
		86400*30,               // maxAge (30天)
		"/",                    // path
		"",                     // domain
		false,                  // secure
		false,                  // httpOnly - 设置为false以允许JavaScript读取
	)

	Success(c, gin.H{
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

// GetAuthConfig 获取认证配置（公开接口，用于登录页面）
func (h *SettingsHandler) GetAuthConfig(c *gin.Context) {
	authConfig := gin.H{
		"auth_enabled":       h.config.Auth.Enabled,
		"username_required":  h.config.Auth.Username != "",
		"auth_username":      h.config.Auth.Username,
		"device_name":        h.config.Server.DeviceName,
	}
	Success(c, authConfig)
}
