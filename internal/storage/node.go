package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateNode 创建节点
func CreateNode(node *ProxyNode) error {
	query := `INSERT INTO proxy_nodes
		(name, type, config, detour_id, tun_name, tun_address, table_id, enabled, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		node.Name, node.Type, node.Config, node.DetourID,
		node.TunName, node.TunAddress, node.TableID,
		node.Enabled, node.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	node.ID = int(id)
	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()

	return nil
}

// GetNode 获取单个节点
func GetNode(id int) (*ProxyNode, error) {
	query := `SELECT id, name, type, config, detour_id, tun_name, tun_address,
		table_id, enabled, status, pid, created_at, updated_at
		FROM proxy_nodes WHERE id = ?`

	node := &ProxyNode{}
	err := db.QueryRow(query, id).Scan(
		&node.ID, &node.Name, &node.Type, &node.Config, &node.DetourID,
		&node.TunName, &node.TunAddress, &node.TableID,
		&node.Enabled, &node.Status, &node.Pid,
		&node.CreatedAt, &node.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return node, nil
}

// GetAllNodes 获取所有节点
func GetAllNodes() ([]*ProxyNode, error) {
	query := `SELECT id, name, type, config, detour_id, tun_name, tun_address,
		table_id, enabled, status, pid, created_at, updated_at
		FROM proxy_nodes ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*ProxyNode
	for rows.Next() {
		node := &ProxyNode{}
		err := rows.Scan(
			&node.ID, &node.Name, &node.Type, &node.Config, &node.DetourID,
			&node.TunName, &node.TunAddress, &node.TableID,
			&node.Enabled, &node.Status, &node.Pid,
			&node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetEnabledNodes 获取所有启用的节点
func GetEnabledNodes() ([]*ProxyNode, error) {
	query := `SELECT id, name, type, config, detour_id, tun_name, tun_address,
		table_id, enabled, status, pid, created_at, updated_at
		FROM proxy_nodes WHERE enabled = 1 ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*ProxyNode
	for rows.Next() {
		node := &ProxyNode{}
		err := rows.Scan(
			&node.ID, &node.Name, &node.Type, &node.Config, &node.DetourID,
			&node.TunName, &node.TunAddress, &node.TableID,
			&node.Enabled, &node.Status, &node.Pid,
			&node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// UpdateNode 更新节点
func UpdateNode(node *ProxyNode) error {
	query := `UPDATE proxy_nodes SET
		name = ?, type = ?, config = ?, detour_id = ?,
		tun_name = ?, tun_address = ?, table_id = ?,
		enabled = ?, status = ?, pid = ?, updated_at = ?
		WHERE id = ?`

	node.UpdatedAt = time.Now()

	result, err := db.Exec(query,
		node.Name, node.Type, node.Config, node.DetourID,
		node.TunName, node.TunAddress, node.TableID,
		node.Enabled, node.Status, node.Pid, node.UpdatedAt,
		node.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found")
	}

	return nil
}

// UpdateNodeStatus 更新节点状态
func UpdateNodeStatus(id int, status string, pid int) error {
	query := `UPDATE proxy_nodes SET status = ?, pid = ?, updated_at = ? WHERE id = ?`

	result, err := db.Exec(query, status, pid, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found")
	}

	return nil
}

// DeleteNode 删除节点
func DeleteNode(id int) error {
	query := `DELETE FROM proxy_nodes WHERE id = ?`

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found")
	}

	return nil
}

// GetNextAvailableTableID 获取下一个可用的 Table ID
func GetNextAvailableTableID() (int, error) {
	query := `SELECT COALESCE(MAX(table_id), 99) + 1 FROM proxy_nodes`

	var nextID int
	err := db.QueryRow(query).Scan(&nextID)
	if err != nil {
		return 0, fmt.Errorf("failed to get next table id: %w", err)
	}

	return nextID, nil
}

// GetTunNameByID 根据节点ID生成TUN名称
func GetTunNameByID(id int) string {
	return fmt.Sprintf("tun%d", id)
}

// GetTunAddressByID 根据节点ID生成TUN地址
func GetTunAddressByID(id int) string {
	// 使用 172.18.0.0/16 网段，每个节点分配 /30 (4个IP)
	base := 172*256*256*256 + 18*256*256 // 172.18.0.0
	offset := (id - 1) * 4
	ip := base + offset + 1

	return fmt.Sprintf("%d.%d.%d.%d/30",
		(ip>>24)&0xFF, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}
