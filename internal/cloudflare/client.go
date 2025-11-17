package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	cloudflareAPIBase = "https://api.cloudflare.com/client/v4"
)

// Client Cloudflare API 客户端
type Client struct {
	apiToken   string
	httpClient *http.Client
}

// Zone Cloudflare 区域
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DNSRecord DNS 记录
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// CloudflareResponse Cloudflare API 响应
type CloudflareResponse struct {
	Success    bool            `json:"success"`
	Errors     []ErrorMessage  `json:"errors"`
	Messages   []string        `json:"messages"`
	Result     json.RawMessage `json:"result"`
	ResultInfo *ResultInfo     `json:"result_info,omitempty"`
}

// ResultInfo 分页信息
type ResultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

// ErrorMessage 错误消息
type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewClient 创建新的 Cloudflare 客户端
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest 执行 API 请求
func (c *Client) doRequest(method, path string, body interface{}) (*CloudflareResponse, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, cloudflareAPIBase+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var cfResp CloudflareResponse
	if err := json.Unmarshal(respBody, &cfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", cfResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	return &cfResp, nil
}

// ListZones 列出所有区域（域名）
func (c *Client) ListZones() ([]Zone, error) {
	return c.ListZonesWithPage(1, 100)
}

// ListZonesWithPage 列出区域（分页）
func (c *Client) ListZonesWithPage(page, perPage int) ([]Zone, error) {
	path := fmt.Sprintf("/zones?page=%d&per_page=%d", page, perPage)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var zones []Zone
	if err := json.Unmarshal(resp.Result, &zones); err != nil {
		return nil, fmt.Errorf("failed to parse zones: %w", err)
	}

	log.Infof("Retrieved %d zones from Cloudflare (page %d)", len(zones), page)
	return zones, nil
}

// ListZonesWithPageInfo 列出区域（分页，返回分页信息）
func (c *Client) ListZonesWithPageInfo(page, perPage int) ([]Zone, *ResultInfo, error) {
	path := fmt.Sprintf("/zones?page=%d&per_page=%d", page, perPage)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	var zones []Zone
	if err := json.Unmarshal(resp.Result, &zones); err != nil {
		return nil, nil, fmt.Errorf("failed to parse zones: %w", err)
	}

	if resp.ResultInfo != nil {
		log.Infof("Retrieved %d zones from Cloudflare (page %d/%d)", len(zones), page, resp.ResultInfo.TotalPages)
	} else {
		log.Infof("Retrieved %d zones from Cloudflare (page %d)", len(zones), page)
	}

	return zones, resp.ResultInfo, nil
}

// DNSRecordsResponse DNS 记录列表响应
type DNSRecordsResponse struct {
	Records    []DNSRecord `json:"records"`
	ResultInfo *ResultInfo `json:"result_info,omitempty"`
}

// ListDNSRecords 列出指定区域的 DNS 记录（支持分页）
func (c *Client) ListDNSRecords(zoneID string) ([]DNSRecord, error) {
	return c.ListDNSRecordsWithPage(zoneID, 1, 100)
}

// ListDNSRecordsWithPage 列出指定区域的 DNS 记录（分页）
func (c *Client) ListDNSRecordsWithPage(zoneID string, page, perPage int) ([]DNSRecord, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?page=%d&per_page=%d", zoneID, page, perPage)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var records []DNSRecord
	if err := json.Unmarshal(resp.Result, &records); err != nil {
		return nil, fmt.Errorf("failed to parse DNS records: %w", err)
	}

	log.Infof("Retrieved %d DNS records for zone %s (page %d)", len(records), zoneID, page)
	return records, nil
}

// ListDNSRecordsWithPageInfo 列出指定区域的 DNS 记录（分页，返回分页信息）
func (c *Client) ListDNSRecordsWithPageInfo(zoneID string, page, perPage int) ([]DNSRecord, *ResultInfo, error) {
	return c.ListDNSRecordsWithFilter(zoneID, page, perPage, "")
}

// ListDNSRecordsWithFilter 列出指定区域的 DNS 记录（支持类型过滤）
func (c *Client) ListDNSRecordsWithFilter(zoneID string, page, perPage int, recordType string) ([]DNSRecord, *ResultInfo, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?page=%d&per_page=%d", zoneID, page, perPage)

	// 如果指定了记录类型，添加到 URL 参数
	if recordType != "" {
		path += fmt.Sprintf("&type=%s", recordType)
	}

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	var records []DNSRecord
	if err := json.Unmarshal(resp.Result, &records); err != nil {
		return nil, nil, fmt.Errorf("failed to parse DNS records: %w", err)
	}

	if resp.ResultInfo != nil {
		log.Infof("Retrieved %d DNS records for zone %s (page %d/%d, type: %s)", len(records), zoneID, page, resp.ResultInfo.TotalPages, recordType)
	} else {
		log.Infof("Retrieved %d DNS records for zone %s (page %d, type: %s)", len(records), zoneID, page, recordType)
	}

	return records, resp.ResultInfo, nil
}

// GetDNSRecord 获取指定的 DNS 记录
func (c *Client) GetDNSRecord(zoneID, recordName, recordType string) (*DNSRecord, error) {
	records, err := c.ListDNSRecords(zoneID)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.Name == recordName && record.Type == recordType {
			return &record, nil
		}
	}

	return nil, fmt.Errorf("DNS record not found: %s (%s)", recordName, recordType)
}

// CreateDNSRecord 创建 DNS 记录
func (c *Client) CreateDNSRecord(zoneID, recordType, name, content string, ttl int, proxied bool) (*DNSRecord, error) {
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

	requestBody := map[string]interface{}{
		"type":    recordType,
		"name":    name,
		"content": content,
		"ttl":     ttl,
		"proxied": proxied,
	}

	resp, err := c.doRequest("POST", path, requestBody)
	if err != nil {
		return nil, err
	}

	var record DNSRecord
	if err := json.Unmarshal(resp.Result, &record); err != nil {
		return nil, fmt.Errorf("failed to parse created DNS record: %w", err)
	}

	log.Infof("Created DNS record: %s (%s) -> %s", name, recordType, content)
	return &record, nil
}

// UpdateDNSRecord 更新 DNS 记录
func (c *Client) UpdateDNSRecord(zoneID, recordID, recordType, name, content string, ttl int, proxied bool) (*DNSRecord, error) {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)

	requestBody := map[string]interface{}{
		"type":    recordType,
		"name":    name,
		"content": content,
		"ttl":     ttl,
		"proxied": proxied,
	}

	resp, err := c.doRequest("PUT", path, requestBody)
	if err != nil {
		return nil, err
	}

	var record DNSRecord
	if err := json.Unmarshal(resp.Result, &record); err != nil {
		return nil, fmt.Errorf("failed to parse updated DNS record: %w", err)
	}

	log.Infof("Updated DNS record: %s (%s) -> %s", name, recordType, content)
	return &record, nil
}

// DeleteDNSRecord 删除 DNS 记录
func (c *Client) DeleteDNSRecord(zoneID, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)

	_, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return err
	}

	log.Infof("Deleted DNS record: %s", recordID)
	return nil
}

// UpdateOrCreateDNSRecord 更新或创建 DNS 记录
func (c *Client) UpdateOrCreateDNSRecord(zoneID, recordType, name, content string, ttl int, proxied bool) (*DNSRecord, error) {
	// 尝试获取现有记录
	existingRecord, err := c.GetDNSRecord(zoneID, name, recordType)
	if err == nil {
		// 记录存在，更新它
		if existingRecord.Content == content {
			log.Infof("DNS record %s (%s) already up to date", name, recordType)
			return existingRecord, nil
		}
		return c.UpdateDNSRecord(zoneID, existingRecord.ID, recordType, name, content, ttl, proxied)
	}

	// 记录不存在，创建它
	return c.CreateDNSRecord(zoneID, recordType, name, content, ttl, proxied)
}

// UpdateDNSRecordByID 根据记录 ID 直接更新 DNS 记录（不查找名称）
func (c *Client) UpdateDNSRecordByID(zoneID, recordID, recordType, name, content string, ttl int, proxied bool) (*DNSRecord, error) {
	return c.UpdateDNSRecord(zoneID, recordID, recordType, name, content, ttl, proxied)
}
