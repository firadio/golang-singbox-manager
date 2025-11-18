package storage

import (
	"fmt"
	"time"
)

// GetAllPortMappings 获取所有端口映射
func GetAllPortMappings() ([]*PortMapping, error) {
	query := `SELECT id, name, protocol, dst_port, to_address, to_port, enable_masquerade, enabled, created_at, updated_at
		FROM port_mappings ORDER BY dst_port ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query port mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*PortMapping
	for rows.Next() {
		mapping := &PortMapping{}
		err := rows.Scan(
			&mapping.ID, &mapping.Name, &mapping.Protocol,
			&mapping.DstPort, &mapping.ToAddress, &mapping.ToPort,
			&mapping.EnableMasquerade, &mapping.Enabled, &mapping.CreatedAt, &mapping.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan port mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// GetEnabledPortMappings 获取所有启用的端口映射
func GetEnabledPortMappings() ([]*PortMapping, error) {
	query := `SELECT id, name, protocol, dst_port, to_address, to_port, enable_masquerade, enabled, created_at, updated_at
		FROM port_mappings WHERE enabled = 1 ORDER BY dst_port ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled port mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*PortMapping
	for rows.Next() {
		mapping := &PortMapping{}
		err := rows.Scan(
			&mapping.ID, &mapping.Name, &mapping.Protocol,
			&mapping.DstPort, &mapping.ToAddress, &mapping.ToPort,
			&mapping.EnableMasquerade, &mapping.Enabled, &mapping.CreatedAt, &mapping.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan port mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// GetPortMapping 根据 ID 获取端口映射
func GetPortMapping(id int) (*PortMapping, error) {
	query := `SELECT id, name, protocol, dst_port, to_address, to_port, enable_masquerade, enabled, created_at, updated_at
		FROM port_mappings WHERE id = ?`

	mapping := &PortMapping{}
	err := db.QueryRow(query, id).Scan(
		&mapping.ID, &mapping.Name, &mapping.Protocol,
		&mapping.DstPort, &mapping.ToAddress, &mapping.ToPort,
		&mapping.EnableMasquerade, &mapping.Enabled, &mapping.CreatedAt, &mapping.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get port mapping: %w", err)
	}

	return mapping, nil
}

// CheckDstPortExists 检查目标端口是否已存在（排除指定ID）
func CheckDstPortExists(dstPort int, excludeID int) (bool, error) {
	query := `SELECT COUNT(*) FROM port_mappings WHERE dst_port = ? AND id != ?`

	var count int
	err := db.QueryRow(query, dstPort, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check dst port: %w", err)
	}

	return count > 0, nil
}

// CreatePortMapping 创建端口映射
func CreatePortMapping(mapping *PortMapping) error {
	// 检查端口是否已存在
	exists, err := CheckDstPortExists(mapping.DstPort, 0)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("destination port %d already exists", mapping.DstPort)
	}

	query := `INSERT INTO port_mappings (name, protocol, dst_port, to_address, to_port, enable_masquerade, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := db.Exec(query,
		mapping.Name, mapping.Protocol, mapping.DstPort,
		mapping.ToAddress, mapping.ToPort, mapping.EnableMasquerade, mapping.Enabled,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create port mapping: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	mapping.ID = int(id)
	mapping.CreatedAt = now
	mapping.UpdatedAt = now

	return nil
}

// UpdatePortMapping 更新端口映射
func UpdatePortMapping(mapping *PortMapping) error {
	// 检查端口是否已被其他记录使用
	exists, err := CheckDstPortExists(mapping.DstPort, mapping.ID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("destination port %d already exists", mapping.DstPort)
	}

	query := `UPDATE port_mappings SET
		name = ?, protocol = ?, dst_port = ?, to_address = ?, to_port = ?, enable_masquerade = ?, enabled = ?, updated_at = ?
		WHERE id = ?`

	now := time.Now()
	_, err = db.Exec(query,
		mapping.Name, mapping.Protocol, mapping.DstPort,
		mapping.ToAddress, mapping.ToPort, mapping.EnableMasquerade, mapping.Enabled,
		now, mapping.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update port mapping: %w", err)
	}

	mapping.UpdatedAt = now
	return nil
}

// DeletePortMapping 删除端口映射
func DeletePortMapping(id int) error {
	query := `DELETE FROM port_mappings WHERE id = ?`

	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete port mapping: %w", err)
	}

	return nil
}
