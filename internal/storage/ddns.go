package storage

import (
	"database/sql"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// DDNSRecord DDNS 记录
type DDNSRecord struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	APIToken          string     `json:"api_token"`
	ZoneID            string     `json:"zone_id"`
	ZoneName          string     `json:"zone_name"`
	RecordName        string     `json:"record_name"`
	RecordType        string     `json:"record_type"`
	DNSRecordID       string     `json:"dns_record_id,omitempty"`
	IPSource          string     `json:"ip_source"`
	IPDetectURL       string     `json:"ip_detect_url,omitempty"`
	MikrotikInterface string     `json:"mikrotik_interface,omitempty"`
	CurrentIP         string     `json:"current_ip,omitempty"`
	LastIP            string     `json:"last_ip,omitempty"`
	LastUpdate        *time.Time `json:"last_update,omitempty"`
	Enabled           bool       `json:"enabled"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CreateDDNSRecord 创建 DDNS 记录
func CreateDDNSRecord(record *DDNSRecord) error {
	query := `
		INSERT INTO ddns_records (
			name, provider, api_token, zone_id, zone_name, record_name, record_type,
			dns_record_id, ip_source, ip_detect_url, mikrotik_interface, current_ip, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := db.Exec(
		query,
		record.Name,
		record.Provider,
		record.APIToken,
		record.ZoneID,
		record.ZoneName,
		record.RecordName,
		record.RecordType,
		record.DNSRecordID,
		record.IPSource,
		record.IPDetectURL,
		record.MikrotikInterface,
		record.CurrentIP,
		record.Enabled,
	)
	if err != nil {
		return fmt.Errorf("failed to create DDNS record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	record.ID = id
	log.Infof("Created DDNS record: %s (ID: %d)", record.Name, id)
	return nil
}

// GetDDNSRecord 获取 DDNS 记录
func GetDDNSRecord(id int64) (*DDNSRecord, error) {
	query := `
		SELECT id, name, provider, api_token, zone_id, zone_name, record_name, record_type,
		       dns_record_id, ip_source, ip_detect_url, mikrotik_interface, current_ip, last_ip, last_update,
		       enabled, created_at, updated_at
		FROM ddns_records
		WHERE id = ?
	`

	record := &DDNSRecord{}
	var lastUpdate sql.NullTime
	var currentIP, lastIP, mikrotikInterface, dnsRecordID, ipDetectURL sql.NullString

	err := db.QueryRow(query, id).Scan(
		&record.ID,
		&record.Name,
		&record.Provider,
		&record.APIToken,
		&record.ZoneID,
		&record.ZoneName,
		&record.RecordName,
		&record.RecordType,
		&dnsRecordID,
		&record.IPSource,
		&ipDetectURL,
		&mikrotikInterface,
		&currentIP,
		&lastIP,
		&lastUpdate,
		&record.Enabled,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("DDNS record not found")
		}
		return nil, fmt.Errorf("failed to get DDNS record: %w", err)
	}

	if lastUpdate.Valid {
		record.LastUpdate = &lastUpdate.Time
	}
	if currentIP.Valid {
		record.CurrentIP = currentIP.String
	}
	if lastIP.Valid {
		record.LastIP = lastIP.String
	}
	if mikrotikInterface.Valid {
		record.MikrotikInterface = mikrotikInterface.String
	}
	if dnsRecordID.Valid {
		record.DNSRecordID = dnsRecordID.String
	}
	if ipDetectURL.Valid {
		record.IPDetectURL = ipDetectURL.String
	}

	return record, nil
}

// GetAllDDNSRecords 获取所有 DDNS 记录
func GetAllDDNSRecords() ([]*DDNSRecord, error) {
	query := `
		SELECT id, name, provider, api_token, zone_id, zone_name, record_name, record_type,
		       dns_record_id, ip_source, ip_detect_url, mikrotik_interface, current_ip, last_ip, last_update,
		       enabled, created_at, updated_at
		FROM ddns_records
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query DDNS records: %w", err)
	}
	defer rows.Close()

	var records []*DDNSRecord
	for rows.Next() {
		record := &DDNSRecord{}
		var lastUpdate sql.NullTime
		var currentIP, lastIP, mikrotikInterface, dnsRecordID, ipDetectURL sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.Name,
			&record.Provider,
			&record.APIToken,
			&record.ZoneID,
			&record.ZoneName,
			&record.RecordName,
			&record.RecordType,
			&dnsRecordID,
			&record.IPSource,
			&ipDetectURL,
			&mikrotikInterface,
			&currentIP,
			&lastIP,
			&lastUpdate,
			&record.Enabled,
			&record.CreatedAt,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan DDNS record: %w", err)
		}

		if lastUpdate.Valid {
			record.LastUpdate = &lastUpdate.Time
		}
		if currentIP.Valid {
			record.CurrentIP = currentIP.String
		}
		if lastIP.Valid {
			record.LastIP = lastIP.String
		}
		if mikrotikInterface.Valid {
			record.MikrotikInterface = mikrotikInterface.String
		}
		if dnsRecordID.Valid {
			record.DNSRecordID = dnsRecordID.String
		}
		if ipDetectURL.Valid {
			record.IPDetectURL = ipDetectURL.String
		}

		records = append(records, record)
	}

	return records, nil
}

// UpdateDDNSRecord 更新 DDNS 记录
func UpdateDDNSRecord(record *DDNSRecord) error {
	query := `
		UPDATE ddns_records
		SET name = ?, provider = ?, api_token = ?, zone_id = ?, zone_name = ?,
		    record_name = ?, record_type = ?, dns_record_id = ?, ip_source = ?, ip_detect_url = ?, mikrotik_interface = ?,
		    current_ip = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := db.Exec(
		query,
		record.Name,
		record.Provider,
		record.APIToken,
		record.ZoneID,
		record.ZoneName,
		record.RecordName,
		record.RecordType,
		record.DNSRecordID,
		record.IPSource,
		record.IPDetectURL,
		record.MikrotikInterface,
		record.CurrentIP,
		record.Enabled,
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update DDNS record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DDNS record not found")
	}

	log.Infof("Updated DDNS record: %s (ID: %d)", record.Name, record.ID)
	return nil
}

// UpdateDDNSRecordIP 更新 DDNS 记录的 IP
func UpdateDDNSRecordIP(id int64, currentIP string) error {
	query := `
		UPDATE ddns_records
		SET last_ip = current_ip, current_ip = ?, last_update = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := db.Exec(query, currentIP, id)
	if err != nil {
		return fmt.Errorf("failed to update DDNS record IP: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DDNS record not found")
	}

	log.Infof("Updated DDNS record IP (ID: %d) to %s", id, currentIP)
	return nil
}

// DeleteDDNSRecord 删除 DDNS 记录
func DeleteDDNSRecord(id int64) error {
	query := `DELETE FROM ddns_records WHERE id = ?`

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete DDNS record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DDNS record not found")
	}

	log.Infof("Deleted DDNS record (ID: %d)", id)
	return nil
}

// EnableDDNSRecord 启用 DDNS 记录
func EnableDDNSRecord(id int64) error {
	query := `UPDATE ddns_records SET enabled = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to enable DDNS record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DDNS record not found")
	}

	log.Infof("Enabled DDNS record (ID: %d)", id)
	return nil
}

// DisableDDNSRecord 禁用 DDNS 记录
func DisableDDNSRecord(id int64) error {
	query := `UPDATE ddns_records SET enabled = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to disable DDNS record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DDNS record not found")
	}

	log.Infof("Disabled DDNS record (ID: %d)", id)
	return nil
}

// GetEnabledDDNSRecords 获取所有启用的 DDNS 记录
func GetEnabledDDNSRecords() ([]*DDNSRecord, error) {
	query := `
		SELECT id, name, provider, api_token, zone_id, zone_name, record_name, record_type,
		       dns_record_id, ip_source, ip_detect_url, mikrotik_interface, current_ip, last_ip, last_update,
		       enabled, created_at, updated_at
		FROM ddns_records
		WHERE enabled = 1
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled DDNS records: %w", err)
	}
	defer rows.Close()

	var records []*DDNSRecord
	for rows.Next() {
		record := &DDNSRecord{}
		var lastUpdate sql.NullTime
		var currentIP, lastIP, mikrotikInterface, dnsRecordID, ipDetectURL sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.Name,
			&record.Provider,
			&record.APIToken,
			&record.ZoneID,
			&record.ZoneName,
			&record.RecordName,
			&record.RecordType,
			&dnsRecordID,
			&record.IPSource,
			&ipDetectURL,
			&mikrotikInterface,
			&currentIP,
			&lastIP,
			&lastUpdate,
			&record.Enabled,
			&record.CreatedAt,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan DDNS record: %w", err)
		}

		if lastUpdate.Valid {
			record.LastUpdate = &lastUpdate.Time
		}
		if currentIP.Valid {
			record.CurrentIP = currentIP.String
		}
		if lastIP.Valid {
			record.LastIP = lastIP.String
		}
		if mikrotikInterface.Valid {
			record.MikrotikInterface = mikrotikInterface.String
		}
		if dnsRecordID.Valid {
			record.DNSRecordID = dnsRecordID.String
		}
		if ipDetectURL.Valid {
			record.IPDetectURL = ipDetectURL.String
		}

		records = append(records, record)
	}

	return records, nil
}
