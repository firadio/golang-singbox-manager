package storage

import (
	"fmt"
	"time"
)

// SaveNodeStats 保存节点统计信息
func SaveNodeStats(stats *NodeStats) error {
	// 先尝试更新
	query := `UPDATE node_stats SET
		latency = ?, http_latency = ?, socks5_latency = ?, available = ?, last_check = ?
		WHERE node_id = ?`

	result, err := db.Exec(query,
		stats.Latency, stats.HTTPLatency, stats.Socks5Latency, stats.Available, stats.LastCheck,
		stats.NodeID,
	)
	if err != nil {
		return fmt.Errorf("failed to update node stats: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// 如果没有更新任何行，说明是新记录，执行插入
	if rows == 0 {
		query = `INSERT INTO node_stats
			(node_id, latency, http_latency, socks5_latency, available, last_check)
			VALUES (?, ?, ?, ?, ?, ?)`

		result, err = db.Exec(query,
			stats.NodeID, stats.Latency, stats.HTTPLatency, stats.Socks5Latency, stats.Available, stats.LastCheck,
		)
		if err != nil {
			return fmt.Errorf("failed to insert node stats: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %w", err)
		}
		stats.ID = int(id)
	}

	return nil
}

// GetNodeStats 获取节点统计信息
func GetNodeStats(nodeID int) (*NodeStats, error) {
	query := `SELECT id, node_id, latency, http_latency, socks5_latency, tx_bytes, rx_bytes, last_check, available
		FROM node_stats WHERE node_id = ?`

	stats := &NodeStats{}
	err := db.QueryRow(query, nodeID).Scan(
		&stats.ID, &stats.NodeID, &stats.Latency, &stats.HTTPLatency, &stats.Socks5Latency,
		&stats.TxBytes, &stats.RxBytes,
		&stats.LastCheck, &stats.Available,
	)

	if err != nil {
		// 如果没有记录，返回默认值
		return &NodeStats{
			NodeID:        nodeID,
			Latency:       0,
			HTTPLatency:   0,
			Socks5Latency: 0,
			Available:     false,
			LastCheck:     time.Time{},
		}, nil
	}

	return stats, nil
}

// GetAllNodeStats 获取所有节点的统计信息
func GetAllNodeStats() (map[int]*NodeStats, error) {
	query := `SELECT id, node_id, latency, http_latency, socks5_latency, tx_bytes, rx_bytes, last_check, available
		FROM node_stats`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query node stats: %w", err)
	}
	defer rows.Close()

	statsMap := make(map[int]*NodeStats)
	for rows.Next() {
		stats := &NodeStats{}
		err := rows.Scan(
			&stats.ID, &stats.NodeID, &stats.Latency, &stats.HTTPLatency, &stats.Socks5Latency,
			&stats.TxBytes, &stats.RxBytes,
			&stats.LastCheck, &stats.Available,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node stats: %w", err)
		}
		statsMap[stats.NodeID] = stats
	}

	return statsMap, nil
}
