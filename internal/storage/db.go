package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

var db *sql.DB

// InitDB 初始化数据库连接
func InitDB(dbPath string) error {
	// 确保数据目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(1) // SQLite 推荐单连接
	db.SetMaxIdleConns(1)

	// 测试连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// 执行数据库迁移
	if err := migrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Info("Database initialized successfully")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

// migrate 执行数据库迁移
func migrate() error {
	migrations := []string{
		// 创建节点表
		`CREATE TABLE IF NOT EXISTS proxy_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			config TEXT NOT NULL,
			detour_id INTEGER,
			tun_name TEXT NOT NULL UNIQUE,
			tun_address TEXT NOT NULL,
			table_id INTEGER NOT NULL UNIQUE,
			enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'stopped',
			pid INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (detour_id) REFERENCES proxy_nodes(id) ON DELETE SET NULL
		)`,

		// 创建路由规则表
		`CREATE TABLE IF NOT EXISTS routing_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			source_ip TEXT NOT NULL,
			source_cidr TEXT,
			node_id INTEGER NOT NULL,
			priority INTEGER NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE,
			UNIQUE(source_ip, node_id)
		)`,

		// 创建节点统计表
		`CREATE TABLE IF NOT EXISTS node_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id INTEGER NOT NULL,
			latency INTEGER DEFAULT 0,
			http_latency INTEGER DEFAULT 0,
			socks5_latency INTEGER DEFAULT 0,
			tx_bytes INTEGER DEFAULT 0,
			rx_bytes INTEGER DEFAULT 0,
			last_check DATETIME,
			available BOOLEAN DEFAULT 0,
			FOREIGN KEY (node_id) REFERENCES proxy_nodes(id) ON DELETE CASCADE
		)`,

		// 创建操作日志表
		`CREATE TABLE IF NOT EXISTS operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation TEXT NOT NULL,
			target_type TEXT,
			target_id INTEGER,
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 创建端口映射表
		`CREATE TABLE IF NOT EXISTS port_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			dst_port INTEGER NOT NULL UNIQUE,
			to_address TEXT NOT NULL,
			to_port INTEGER NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 创建DDNS配置表
		`CREATE TABLE IF NOT EXISTS ddns_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'cloudflare',
			api_token TEXT NOT NULL,
			zone_id TEXT NOT NULL,
			zone_name TEXT NOT NULL,
			record_name TEXT NOT NULL,
			record_type TEXT NOT NULL DEFAULT 'A',
			dns_record_id TEXT,
			ip_source TEXT NOT NULL DEFAULT 'public',
			ip_detect_url TEXT DEFAULT 'https://api.ip.sb/ip',
			mikrotik_interface TEXT,
			current_ip TEXT,
			last_ip TEXT,
			last_update DATETIME,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(zone_id, record_name, record_type, dns_record_id)
		)`,

		// 创建索引
		`CREATE INDEX IF NOT EXISTS idx_routing_rules_node_id ON routing_rules(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_routing_rules_enabled ON routing_rules(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_node_stats_node_id ON node_stats(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_port_mappings_dst_port ON port_mappings(dst_port)`,
		`CREATE INDEX IF NOT EXISTS idx_port_mappings_enabled ON port_mappings(enabled)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	log.Info("Database migration completed")

	// 添加新列（如果不存在）
	if err := addColumnsIfNotExist(); err != nil {
		return fmt.Errorf("failed to add new columns: %w", err)
	}

	// 添加统计表的新列
	if err := addStatsColumnsIfNotExist(); err != nil {
		return fmt.Errorf("failed to add stats columns: %w", err)
	}

	return nil
}

// addColumnsIfNotExist 添加新列（如果不存在）
func addColumnsIfNotExist() error {
	// 检查 proxy_nodes 表中是否存在新列
	columns := []struct {
		name         string
		definition   string
		defaultValue string
	}{
		{"inbound_type", "TEXT DEFAULT 'tun'", "'tun'"},
		{"inbound_listen", "TEXT DEFAULT '127.0.0.1'", "'127.0.0.1'"},
		{"inbound_port", "INTEGER", "NULL"},
		{"hijack_dns", "BOOLEAN DEFAULT 1", "1"},
	}

	// 添加 DDNS 表的新列
	ddnsColumns := []struct {
		table      string
		name       string
		definition string
	}{
		{"ddns_records", "ip_detect_url", "TEXT DEFAULT 'https://api.ip.sb/ip'"},
		{"port_mappings", "enable_masquerade", "BOOLEAN DEFAULT 0"},
	}

	// 获取现有列
	rows, err := db.Query("PRAGMA table_info(proxy_nodes)")
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		existingColumns[name] = true
	}

	// 添加不存在的列
	for _, col := range columns {
		if !existingColumns[col.name] {
			alterSQL := fmt.Sprintf("ALTER TABLE proxy_nodes ADD COLUMN %s %s", col.name, col.definition)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add column %s: %w", col.name, err)
			}
			log.Infof("Added column %s to proxy_nodes table", col.name)
		}
	}

	// 为 DDNS 表添加新列
	for _, col := range ddnsColumns {
		// 检查列是否存在
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", col.table))
		if err != nil {
			continue // 表可能不存在，跳过
		}

		existingCols := make(map[string]bool)
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue sql.NullString
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				rows.Close()
				continue
			}
			existingCols[name] = true
		}
		rows.Close()

		if !existingCols[col.name] {
			alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", col.table, col.name, col.definition)
			if _, err := db.Exec(alterSQL); err != nil {
				log.Warnf("Failed to add column %s to %s: %v", col.name, col.table, err)
			} else {
				log.Infof("Added column %s to %s table", col.name, col.table)
			}
		}
	}

	return nil
}

// addStatsColumnsIfNotExist 添加统计表新列（如果不存在）
func addStatsColumnsIfNotExist() error {
	columns := []struct {
		name       string
		definition string
	}{
		{"http_latency", "INTEGER DEFAULT 0"},
		{"socks5_latency", "INTEGER DEFAULT 0"},
	}

	// 获取现有列
	rows, err := db.Query("PRAGMA table_info(node_stats)")
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		existingColumns[name] = true
	}

	// 添加不存在的列
	for _, col := range columns {
		if !existingColumns[col.name] {
			alterSQL := fmt.Sprintf("ALTER TABLE node_stats ADD COLUMN %s %s", col.name, col.definition)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add column %s: %w", col.name, err)
			}
			log.Infof("Added column %s to node_stats table", col.name)
		}
	}

	return nil
}
