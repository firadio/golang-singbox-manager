package handlers

import (
	"strconv"

	"github.com/firadio/golang-singbox-manager/internal/cloudflare"
	"github.com/firadio/golang-singbox-manager/internal/ipdetect"
	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// DDNSHandler DDNS 处理器
type DDNSHandler struct {
	mtClient *mikrotik.Client
}

// NewDDNSHandler 创建 DDNS 处理器
func NewDDNSHandler(mtClient *mikrotik.Client) *DDNSHandler {
	return &DDNSHandler{
		mtClient: mtClient,
	}
}

// GetAllDDNSRecords 获取所有 DDNS 记录
func (h *DDNSHandler) GetAllDDNSRecords(c *gin.Context) {
	records, err := storage.GetAllDDNSRecords()
	if err != nil {
		Error(c, 7001, "Failed to get DDNS records: "+err.Error())
		return
	}

	Success(c, records)
}

// GetDDNSRecord 获取指定 DDNS 记录
func (h *DDNSHandler) GetDDNSRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7002, "Invalid DDNS record ID")
		return
	}

	record, err := storage.GetDDNSRecord(id)
	if err != nil {
		Error(c, 7003, "Failed to get DDNS record: "+err.Error())
		return
	}

	Success(c, record)
}

// CreateDDNSRecord 创建 DDNS 记录
func (h *DDNSHandler) CreateDDNSRecord(c *gin.Context) {
	var record storage.DDNSRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		Error(c, 7004, "Invalid request body: "+err.Error())
		return
	}

	// 验证必填字段
	if record.Name == "" || record.APIToken == "" || record.ZoneID == "" ||
		record.RecordName == "" || record.RecordType == "" || record.IPSource == "" {
		Error(c, 7005, "Missing required fields")
		return
	}

	// 如果是接口 IP，验证 Mikrotik 接口
	if record.IPSource == "interface" && record.MikrotikInterface == "" {
		Error(c, 7006, "Mikrotik interface is required for interface IP source")
		return
	}

	// 验证 Cloudflare API Token（通过尝试获取 zones）
	cfClient := cloudflare.NewClient(record.APIToken)
	if _, err := cfClient.ListZones(); err != nil {
		Error(c, 7007, "Failed to verify Cloudflare API token: "+err.Error())
		return
	}

	// current_ip 由前端传入（前端加载 DNS 记录时已从 Cloudflare 获取）
	log.Infof("Creating DDNS record: %s (current IP: %s)", record.Name, record.CurrentIP)

	// 创建记录
	if err := storage.CreateDDNSRecord(&record); err != nil {
		Error(c, 7008, "Failed to create DDNS record: "+err.Error())
		return
	}

	log.Infof("Created DDNS record: %s (current IP: %s)", record.Name, record.CurrentIP)
	Success(c, record)
}

// UpdateDDNSRecord 更新 DDNS 记录
func (h *DDNSHandler) UpdateDDNSRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7009, "Invalid DDNS record ID")
		return
	}

	var record storage.DDNSRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		Error(c, 7010, "Invalid request body: "+err.Error())
		return
	}

	record.ID = id

	// 验证必填字段
	if record.Name == "" || record.APIToken == "" || record.ZoneID == "" ||
		record.RecordName == "" || record.RecordType == "" || record.IPSource == "" {
		Error(c, 7011, "Missing required fields")
		return
	}

	// 如果是接口 IP，验证 Mikrotik 接口
	if record.IPSource == "interface" && record.MikrotikInterface == "" {
		Error(c, 7012, "Mikrotik interface is required for interface IP source")
		return
	}

	// 验证 Cloudflare API Token（通过尝试获取 zones）
	cfClient := cloudflare.NewClient(record.APIToken)
	if _, err := cfClient.ListZones(); err != nil {
		Error(c, 7013, "Failed to verify Cloudflare API token: "+err.Error())
		return
	}

	// current_ip 由前端传入（前端加载 DNS 记录时已从 Cloudflare 获取）
	log.Infof("Updating DDNS record: %s (current IP: %s)", record.Name, record.CurrentIP)

	// 更新记录
	if err := storage.UpdateDDNSRecord(&record); err != nil {
		Error(c, 7014, "Failed to update DDNS record: "+err.Error())
		return
	}

	log.Infof("Updated DDNS record: %s (current IP: %s)", record.Name, record.CurrentIP)
	Success(c, record)
}

// DeleteDDNSRecord 删除 DDNS 记录
func (h *DDNSHandler) DeleteDDNSRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7015, "Invalid DDNS record ID")
		return
	}

	if err := storage.DeleteDDNSRecord(id); err != nil {
		Error(c, 7016, "Failed to delete DDNS record: "+err.Error())
		return
	}

	log.Infof("Deleted DDNS record (ID: %d)", id)
	Success(c, gin.H{"message": "DDNS record deleted successfully"})
}

// EnableDDNSRecord 启用 DDNS 记录
func (h *DDNSHandler) EnableDDNSRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7017, "Invalid DDNS record ID")
		return
	}

	if err := storage.EnableDDNSRecord(id); err != nil {
		Error(c, 7018, "Failed to enable DDNS record: "+err.Error())
		return
	}

	log.Infof("Enabled DDNS record (ID: %d)", id)
	Success(c, gin.H{"message": "DDNS record enabled successfully"})
}

// DisableDDNSRecord 禁用 DDNS 记录
func (h *DDNSHandler) DisableDDNSRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7019, "Invalid DDNS record ID")
		return
	}

	if err := storage.DisableDDNSRecord(id); err != nil {
		Error(c, 7020, "Failed to disable DDNS record: "+err.Error())
		return
	}

	log.Infof("Disabled DDNS record (ID: %d)", id)
	Success(c, gin.H{"message": "DDNS record disabled successfully"})
}

// GetCloudflareZones 获取 Cloudflare 区域列表（支持分页）
func (h *DDNSHandler) GetCloudflareZones(c *gin.Context) {
	apiToken := c.Query("api_token")
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "20")

	if apiToken == "" {
		Error(c, 7024, "API token is required")
		return
	}

	// 解析分页参数
	pageNum := 1
	perPageNum := 20
	if p, err := strconv.Atoi(page); err == nil && p > 0 {
		pageNum = p
	}
	if pp, err := strconv.Atoi(perPage); err == nil && pp > 0 && pp <= 100 {
		perPageNum = pp
	}

	cfClient := cloudflare.NewClient(apiToken)
	zones, resultInfo, err := cfClient.ListZonesWithPageInfo(pageNum, perPageNum)
	if err != nil {
		Error(c, 7025, "Failed to get Cloudflare zones: "+err.Error())
		return
	}

	// 返回区域列表和分页信息
	Success(c, gin.H{
		"zones":       zones,
		"result_info": resultInfo,
	})
}

// GetCloudflareDNSRecords 获取 Cloudflare DNS 记录列表（支持分页）
func (h *DDNSHandler) GetCloudflareDNSRecords(c *gin.Context) {
	apiToken := c.Query("api_token")
	zoneID := c.Query("zone_id")
	recordType := c.Query("record_type")
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "20")

	if apiToken == "" || zoneID == "" {
		Error(c, 7026, "API token and zone ID are required")
		return
	}

	// 解析分页参数
	pageNum := 1
	perPageNum := 20
	if p, err := strconv.Atoi(page); err == nil && p > 0 {
		pageNum = p
	}
	if pp, err := strconv.Atoi(perPage); err == nil && pp > 0 && pp <= 100 {
		perPageNum = pp
	}

	cfClient := cloudflare.NewClient(apiToken)

	// 获取所有记录，然后在后端过滤
	// 注意：如果记录类型过滤后数量较少，可能需要多次翻页才能获取足够数据
	// 但为了保持简单和兼容性，暂时采用这种方式
	allRecords, _, err := cfClient.ListDNSRecordsWithPageInfo(zoneID, pageNum, perPageNum)
	if err != nil {
		Error(c, 7027, "Failed to get DNS records: "+err.Error())
		return
	}

	// 如果指定了记录类型，过滤记录
	filteredRecords := allRecords
	if recordType != "" {
		filteredRecords = []cloudflare.DNSRecord{}
		for _, record := range allRecords {
			if record.Type == recordType {
				filteredRecords = append(filteredRecords, record)
			}
		}
	}

	// 直接返回记录数组（保持与之前的兼容性）
	Success(c, filteredRecords)
}

// TestPublicIP 测试获取公网 IP
func (h *DDNSHandler) TestPublicIP(c *gin.Context) {
	recordType := c.Query("record_type")
	detectURL := c.Query("detect_url")

	if recordType == "" {
		recordType = "A" // 默认 IPv4
	}

	ip, err := ipdetect.GetIPWithURL("public", recordType, detectURL, nil, "")
	if err != nil {
		Error(c, 7040, "Failed to get public IP: "+err.Error())
		return
	}

	Success(c, gin.H{"ip": ip})
}

// GetMikrotikInterfaces 获取 Mikrotik 接口列表
func (h *DDNSHandler) GetMikrotikInterfaces(c *gin.Context) {
	if h.mtClient == nil {
		Error(c, 7028, "Mikrotik is not configured")
		return
	}

	recordType := c.Query("record_type")
	isIPv6 := recordType == "AAAA"

	var addresses []mikrotik.Address
	var err error

	if isIPv6 {
		// 获取 IPv6 地址
		addresses, err = h.mtClient.GetIPv6Addresses()
	} else {
		// 获取 IPv4 地址
		addresses, err = h.mtClient.GetAddresses()
	}

	if err != nil {
		log.Warnf("Failed to get Mikrotik interfaces: %v", err)
		// 返回空列表而不是错误
		Success(c, []gin.H{})
		return
	}

	// 提取动态接口列表
	interfaceMap := make(map[string]bool)
	interfaces := []gin.H{} // 初始化为空数组，避免返回 null
	for _, addr := range addresses {
		if addr.Dynamic && !interfaceMap[addr.Interface] {
			interfaceMap[addr.Interface] = true
			interfaces = append(interfaces, gin.H{
				"name":    addr.Interface,
				"address": addr.Address,
			})
		}
	}

	log.Infof("Returning %d Mikrotik interfaces for record type %s", len(interfaces), recordType)
	Success(c, interfaces)
}

// UpdateDDNS 手动更新指定 DDNS 记录
func (h *DDNSHandler) UpdateDDNS(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, 7030, "Invalid DDNS record ID")
		return
	}

	record, err := storage.GetDDNSRecord(id)
	if err != nil {
		Error(c, 7031, "Failed to get DDNS record: "+err.Error())
		return
	}

	if !record.Enabled {
		Error(c, 7032, "DDNS record is disabled")
		return
	}

	// 获取当前 IP
	currentIP, err := ipdetect.GetIPWithURL(record.IPSource, record.RecordType, record.IPDetectURL, h.mtClient, record.MikrotikInterface)
	if err != nil {
		Error(c, 7033, "Failed to get current IP: "+err.Error())
		return
	}

	// 检查 IP 是否变化
	if record.CurrentIP == currentIP {
		Success(c, gin.H{
			"message": "IP has not changed",
			"ip":      currentIP,
		})
		return
	}

	// 更新 Cloudflare DNS 记录
	cfClient := cloudflare.NewClient(record.APIToken)
	var dnsRecord *cloudflare.DNSRecord

	if record.DNSRecordID != "" {
		// 如果有记录 ID，直接使用 ID 更新（更精确，避免名称冲突）
		dnsRecord, err = cfClient.UpdateDNSRecordByID(
			record.ZoneID,
			record.DNSRecordID,
			record.RecordType,
			record.RecordName,
			currentIP,
			300, // TTL: 5 minutes
			false,
		)
		if err != nil {
			// 如果使用 ID 更新失败，可能记录被删除了，尝试创建新记录
			log.Warnf("Failed to update DNS record by ID %s: %v, trying to create new record", record.DNSRecordID, err)
			dnsRecord, err = cfClient.UpdateOrCreateDNSRecord(
				record.ZoneID,
				record.RecordType,
				record.RecordName,
				currentIP,
				300,
				false,
			)
			if err != nil {
				Error(c, 7034, "Failed to create DNS record after ID update failed: "+err.Error())
				return
			}
			// 更新数据库中的 record ID
			record.DNSRecordID = dnsRecord.ID
			if err := storage.UpdateDDNSRecord(record); err != nil {
				log.Warnf("Failed to update dns_record_id in database: %v", err)
			}
		}
	} else {
		// 没有记录 ID，使用名称查找并更新或创建
		dnsRecord, err = cfClient.UpdateOrCreateDNSRecord(
			record.ZoneID,
			record.RecordType,
			record.RecordName,
			currentIP,
			300,
			false,
		)
		if err != nil {
			Error(c, 7034, "Failed to update/create DNS record: "+err.Error())
			return
		}
		// 保存新创建的记录 ID
		if dnsRecord.ID != "" && record.DNSRecordID == "" {
			record.DNSRecordID = dnsRecord.ID
			if err := storage.UpdateDDNSRecord(record); err != nil {
				log.Warnf("Failed to save dns_record_id in database: %v", err)
			}
		}
	}

	// 更新数据库中的 IP
	if err := storage.UpdateDDNSRecordIP(id, currentIP); err != nil {
		Error(c, 7035, "Failed to update DDNS record IP: "+err.Error())
		return
	}

	log.Infof("Updated DDNS record %s: %s -> %s", record.Name, record.CurrentIP, currentIP)
	Success(c, gin.H{
		"message": "DDNS record updated successfully",
		"old_ip":  record.CurrentIP,
		"new_ip":  currentIP,
	})
}
