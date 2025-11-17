package ddns

import (
	"sync"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/cloudflare"
	"github.com/firadio/golang-singbox-manager/internal/ipdetect"
	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

// Updater DDNS 自动更新器
type Updater struct {
	mtClient *mikrotik.Client
	interval time.Duration
	ticker   *time.Ticker
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewUpdater 创建 DDNS 更新器
func NewUpdater(mtClient *mikrotik.Client, interval time.Duration) *Updater {
	return &Updater{
		mtClient: mtClient,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动 DDNS 更新器
func (u *Updater) Start() {
	log.Infof("Starting DDNS updater with interval: %v", u.interval)
	u.ticker = time.NewTicker(u.interval)
	u.wg.Add(1)

	go func() {
		defer u.wg.Done()
		for {
			select {
			case <-u.ticker.C:
				u.checkAndUpdate()
			case <-u.stopCh:
				log.Info("DDNS updater stopped")
				return
			}
		}
	}()

	log.Info("DDNS updater started")
}

// Stop 停止 DDNS 更新器
func (u *Updater) Stop() {
	if u.ticker != nil {
		u.ticker.Stop()
	}
	close(u.stopCh)
	u.wg.Wait()
}

// checkAndUpdate 检查并更新所有启用的 DDNS 记录
func (u *Updater) checkAndUpdate() {
	records, err := storage.GetEnabledDDNSRecords()
	if err != nil {
		log.Errorf("Failed to get enabled DDNS records: %v", err)
		return
	}

	if len(records) == 0 {
		log.Debugf("No enabled DDNS records to check")
		return
	}

	log.Infof("Checking %d enabled DDNS records for IP changes", len(records))

	for _, record := range records {
		if err := u.updateRecord(record); err != nil {
			log.Errorf("Failed to update DDNS record %s: %v", record.Name, err)
		}
	}
}

// updateRecord 更新单个 DDNS 记录
func (u *Updater) updateRecord(record *storage.DDNSRecord) error {
	// 获取当前 IP
	currentIP, err := ipdetect.GetIPWithURL(
		record.IPSource,
		record.RecordType,
		record.IPDetectURL,
		u.mtClient,
		record.MikrotikInterface,
	)
	if err != nil {
		return err
	}

	// 比对数据库中的 IP
	if record.CurrentIP == currentIP {
		log.Infof("DDNS record %s (%s): IP unchanged (%s)", record.Name, record.RecordName, currentIP)
		return nil
	}

	log.Infof("DDNS record %s: IP changed from %s to %s, updating Cloudflare...", record.Name, record.CurrentIP, currentIP)

	// IP 有变化，更新 Cloudflare DNS 记录
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
				return err
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
			return err
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
	if err := storage.UpdateDDNSRecordIP(record.ID, currentIP); err != nil {
		return err
	}

	log.Infof("Successfully updated DDNS record %s: %s -> %s", record.Name, record.CurrentIP, currentIP)
	return nil
}
