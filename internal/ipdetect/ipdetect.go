package ipdetect

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPublicIPServiceURL = "https://api.ip.sb/ip"
)

// GetPublicIP 获取公网 IPv4 地址（使用默认源）
func GetPublicIP() (string, error) {
	return GetPublicIPFromURL(defaultPublicIPServiceURL, "tcp4")
}

// GetPublicIPv6 获取公网 IPv6 地址（使用默认源）
func GetPublicIPv6() (string, error) {
	return GetPublicIPFromURL(defaultPublicIPServiceURL, "tcp6")
}

// GetPublicIPFromURL 从指定 URL 获取公网 IP
func GetPublicIPFromURL(url string, network string) (string, error) {
	if url == "" {
		url = defaultPublicIPServiceURL
	}
	return getPublicIPWithDialer(url, network)
}

// getPublicIPWithDialer 使用指定的网络类型和 URL 获取公网 IP
func getPublicIPWithDialer(url string, network string) (string, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get public IP from %s (%s): %w", url, network, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get public IP: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	ip, err := parseIPResponse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse IP response from %s: %w", url, err)
	}

	// 验证 IP 地址格式
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	// 验证 IPv4/IPv6 类型是否匹配
	if network == "tcp4" && parsedIP.To4() == nil {
		return "", fmt.Errorf("expected IPv4 but got IPv6: %s", ip)
	}
	if network == "tcp6" && parsedIP.To4() != nil {
		return "", fmt.Errorf("expected IPv6 but got IPv4: %s", ip)
	}

	log.Infof("Detected public IP from %s (%s): %s", url, network, ip)
	return ip, nil
}

// parseIPResponse 解析不同格式的 IP 响应（纯文本或 JSON）
func parseIPResponse(body []byte) (string, error) {
	bodyStr := strings.TrimSpace(string(body))

	// 尝试作为纯文本 IP 解析
	if parsedIP := net.ParseIP(bodyStr); parsedIP != nil {
		return bodyStr, nil
	}

	// 尝试作为 JSON 解析
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(body, &jsonResponse); err == nil {
		// 常见的 JSON 字段名
		possibleFields := []string{"ip", "IP", "address", "addr", "query"}
		for _, field := range possibleFields {
			if ipValue, ok := jsonResponse[field]; ok {
				if ipStr, ok := ipValue.(string); ok {
					ipStr = strings.TrimSpace(ipStr)
					if parsedIP := net.ParseIP(ipStr); parsedIP != nil {
						return ipStr, nil
					}
				}
			}
		}
		return "", fmt.Errorf("no valid IP field found in JSON response: %s", bodyStr)
	}

	// 无法解析
	return "", fmt.Errorf("unable to parse IP from response: %s", bodyStr)
}

// GetMikrotikInterfaceIP 获取 Mikrotik 接口的动态 IPv4 地址
func GetMikrotikInterfaceIP(client *mikrotik.Client, interfaceName string) (string, error) {
	return getMikrotikInterfaceIPByType(client, interfaceName, false)
}

// GetMikrotikInterfaceIPv6 获取 Mikrotik 接口的动态 IPv6 地址
func GetMikrotikInterfaceIPv6(client *mikrotik.Client, interfaceName string) (string, error) {
	return getMikrotikInterfaceIPByType(client, interfaceName, true)
}

// getMikrotikInterfaceIPByType 根据类型获取 Mikrotik 接口的动态 IP
func getMikrotikInterfaceIPByType(client *mikrotik.Client, interfaceName string, isIPv6 bool) (string, error) {
	if client == nil {
		return "", fmt.Errorf("mikrotik client is nil")
	}

	var addresses []mikrotik.Address
	var err error

	if isIPv6 {
		// 获取 IPv6 地址
		addresses, err = client.GetIPv6Addresses()
	} else {
		// 获取 IPv4 地址
		addresses, err = client.GetAddresses()
	}

	if err != nil {
		return "", fmt.Errorf("failed to get addresses from Mikrotik: %w", err)
	}

	// 查找指定接口的动态地址
	for _, addr := range addresses {
		if addr.Interface == interfaceName && addr.Dynamic {
			// 移除 CIDR 后缀
			ip := strings.Split(addr.Address, "/")[0]

			// 验证 IP 地址格式
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				continue
			}

			// 验证 IP 类型
			if isIPv6 && parsedIP.To4() != nil {
				continue // 期望 IPv6 但得到 IPv4
			}
			if !isIPv6 && parsedIP.To4() == nil {
				continue // 期望 IPv4 但得到 IPv6
			}

			ipType := "IPv4"
			if isIPv6 {
				ipType = "IPv6"
			}
			log.Infof("Detected Mikrotik interface %s: %s on %s", ipType, ip, interfaceName)
			return ip, nil
		}
	}

	ipType := "IPv4"
	if isIPv6 {
		ipType = "IPv6"
	}
	return "", fmt.Errorf("no dynamic %s found for interface %s", ipType, interfaceName)
}

// GetIP 根据来源和记录类型获取 IP 地址
func GetIP(source string, recordType string, mtClient *mikrotik.Client, interfaceName string) (string, error) {
	return GetIPWithURL(source, recordType, "", mtClient, interfaceName)
}

// GetIPWithURL 根据来源、记录类型和自定义 URL 获取 IP 地址
func GetIPWithURL(source string, recordType string, detectURL string, mtClient *mikrotik.Client, interfaceName string) (string, error) {
	isIPv6 := recordType == "AAAA"
	network := "tcp4"
	if isIPv6 {
		network = "tcp6"
	}

	switch source {
	case "public":
		if detectURL == "" {
			detectURL = defaultPublicIPServiceURL
		}
		return GetPublicIPFromURL(detectURL, network)
	case "interface":
		if interfaceName == "" {
			return "", fmt.Errorf("interface name is required for interface IP source")
		}
		if isIPv6 {
			return GetMikrotikInterfaceIPv6(mtClient, interfaceName)
		}
		return GetMikrotikInterfaceIP(mtClient, interfaceName)
	default:
		return "", fmt.Errorf("invalid IP source: %s", source)
	}
}
