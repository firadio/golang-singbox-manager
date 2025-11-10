package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 系统配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	SingBox  SingBoxConfig  `yaml:"singbox"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
	Mikrotik MikrotikConfig `yaml:"mikrotik"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// SingBoxConfig sing-box 配置
type SingBoxConfig struct {
	BinPath   string `yaml:"bin_path"`
	ConfigDir string `yaml:"config_dir"`
	LogDir    string `yaml:"log_dir"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"` // 用户名，留空表示无需用户名
	Password string `yaml:"password"`
}

// MikrotikConfig Mikrotik配置
type MikrotikConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Address     string `yaml:"address"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Port        int    `yaml:"port"`
	RoutingTable string `yaml:"routing_table"` // 路由表名称
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		SingBox: SingBoxConfig{
			BinPath:   "/usr/local/bin/sing-box",
			ConfigDir: "configs/singbox",
			LogDir:    "logs/singbox",
		},
		Database: DatabaseConfig{
			Path: "data/manager.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Auth: AuthConfig{
			Enabled:  false,
			Password: "",
		},
		Mikrotik: MikrotikConfig{
			Enabled:      false,
			Address:      "",
			Username:     "admin",
			Password:     "",
			Port:         8728,
			RoutingTable: "main",
		},
	}
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 如果配置文件不存在，使用默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 将相对路径转换为绝对路径
	if !filepath.IsAbs(config.SingBox.ConfigDir) {
		config.SingBox.ConfigDir, _ = filepath.Abs(config.SingBox.ConfigDir)
	}
	if !filepath.IsAbs(config.SingBox.LogDir) {
		config.SingBox.LogDir, _ = filepath.Abs(config.SingBox.LogDir)
	}
	if !filepath.IsAbs(config.Database.Path) {
		config.Database.Path, _ = filepath.Abs(config.Database.Path)
	}

	return config, nil
}

// SaveConfig 保存配置文件
func SaveConfig(configPath string, config *Config) error {
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
