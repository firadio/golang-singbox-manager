package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateRule 创建路由规则
func CreateRule(rule *RoutingRule) error {
	query := `INSERT INTO routing_rules
		(name, source_ip, source_cidr, node_id, priority, enabled)
		VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query,
		rule.Name, rule.SourceIP, rule.SourceCIDR,
		rule.NodeID, rule.Priority, rule.Enabled,
	)
	if err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	rule.ID = int(id)
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	return nil
}

// GetRule 获取单个规则
func GetRule(id int) (*RoutingRule, error) {
	query := `SELECT id, name, source_ip, source_cidr, node_id, priority,
		enabled, created_at, updated_at
		FROM routing_rules WHERE id = ?`

	rule := &RoutingRule{}
	err := db.QueryRow(query, id).Scan(
		&rule.ID, &rule.Name, &rule.SourceIP, &rule.SourceCIDR,
		&rule.NodeID, &rule.Priority, &rule.Enabled,
		&rule.CreatedAt, &rule.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("rule not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return rule, nil
}

// GetAllRules 获取所有规则
func GetAllRules() ([]*RoutingRule, error) {
	query := `SELECT id, name, source_ip, source_cidr, node_id, priority,
		enabled, created_at, updated_at
		FROM routing_rules ORDER BY priority`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rules: %w", err)
	}
	defer rows.Close()

	var rules []*RoutingRule
	for rows.Next() {
		rule := &RoutingRule{}
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.SourceIP, &rule.SourceCIDR,
			&rule.NodeID, &rule.Priority, &rule.Enabled,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// GetEnabledRules 获取所有启用的规则
func GetEnabledRules() ([]*RoutingRule, error) {
	query := `SELECT id, name, source_ip, source_cidr, node_id, priority,
		enabled, created_at, updated_at
		FROM routing_rules WHERE enabled = 1 ORDER BY priority`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled rules: %w", err)
	}
	defer rows.Close()

	var rules []*RoutingRule
	for rows.Next() {
		rule := &RoutingRule{}
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.SourceIP, &rule.SourceCIDR,
			&rule.NodeID, &rule.Priority, &rule.Enabled,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// GetRulesByNodeID 根据节点ID获取规则
func GetRulesByNodeID(nodeID int) ([]*RoutingRule, error) {
	query := `SELECT id, name, source_ip, source_cidr, node_id, priority,
		enabled, created_at, updated_at
		FROM routing_rules WHERE node_id = ? ORDER BY priority`

	rows, err := db.Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query rules by node: %w", err)
	}
	defer rows.Close()

	var rules []*RoutingRule
	for rows.Next() {
		rule := &RoutingRule{}
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.SourceIP, &rule.SourceCIDR,
			&rule.NodeID, &rule.Priority, &rule.Enabled,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdateRule 更新规则
func UpdateRule(rule *RoutingRule) error {
	query := `UPDATE routing_rules SET
		name = ?, source_ip = ?, source_cidr = ?, node_id = ?,
		priority = ?, enabled = ?, updated_at = ?
		WHERE id = ?`

	rule.UpdatedAt = time.Now()

	result, err := db.Exec(query,
		rule.Name, rule.SourceIP, rule.SourceCIDR,
		rule.NodeID, rule.Priority, rule.Enabled, rule.UpdatedAt,
		rule.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("rule not found")
	}

	return nil
}

// DeleteRule 删除规则
func DeleteRule(id int) error {
	query := `DELETE FROM routing_rules WHERE id = ?`

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("rule not found")
	}

	return nil
}

// GetNextAvailablePriority 获取下一个可用的优先级
func GetNextAvailablePriority() (int, error) {
	query := `SELECT COALESCE(MAX(priority), 99) + 1 FROM routing_rules`

	var nextPriority int
	err := db.QueryRow(query).Scan(&nextPriority)
	if err != nil {
		return 0, fmt.Errorf("failed to get next priority: %w", err)
	}

	return nextPriority, nil
}

// CheckSourceIPExists 检查源IP是否已存在（排除指定ID）
func CheckSourceIPExists(sourceIP string, excludeID int) (bool, error) {
	query := `SELECT COUNT(*) FROM routing_rules WHERE source_ip = ? AND id != ?`

	var count int
	err := db.QueryRow(query, sourceIP, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check source IP: %w", err)
	}

	return count > 0, nil
}
