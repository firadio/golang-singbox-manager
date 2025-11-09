package network

import (
	"fmt"
	"net"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/mikrotik"
	"github.com/vishvananda/netlink"
	log "github.com/sirupsen/logrus"
)

// RoutingManager 路由管理器
type RoutingManager struct {
	mtClient *mikrotik.Client
}

// NewRoutingManager 创建路由管理器
func NewRoutingManager() *RoutingManager {
	return &RoutingManager{}
}

// SetMikrotikClient 设置Mikrotik客户端
func (rm *RoutingManager) SetMikrotikClient(client *mikrotik.Client) {
	rm.mtClient = client
}

// AddRule 添加策略路由规则
func (rm *RoutingManager) AddRule(sourceIP string, tableID int, priority int, ruleID int) error {
	log.Infof("Adding routing rule: %s -> table %d (priority %d, ruleID %d)", sourceIP, tableID, priority, ruleID)

	// 先在Mikrotik上添加规则（将源IP路由到本机）
	if rm.mtClient != nil {
		if err := rm.mtClient.AddRoutingRule(sourceIP, ruleID); err != nil {
			log.Errorf("Failed to add Mikrotik routing rule: %v", err)
			// 继续执行本地规则添加
		}
	}

	// 解析源 IP
	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return fmt.Errorf("invalid source IP: %s", sourceIP)
	}

	// 创建路由规则
	rule := netlink.NewRule()
	rule.Src = &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(32, 32), // /32 表示单个 IP
	}
	rule.Table = tableID
	rule.Priority = priority

	// 添加规则
	if err := netlink.RuleAdd(rule); err != nil {
		return fmt.Errorf("failed to add rule: %w", err)
	}

	log.Infof("Routing rule added successfully")
	return nil
}

// DeleteRule 删除策略路由规则
func (rm *RoutingManager) DeleteRule(sourceIP string, tableID int, ruleID int) error {
	log.Infof("Deleting routing rule: %s -> table %d (ruleID %d)", sourceIP, tableID, ruleID)

	// 先在Mikrotik上删除规则
	if rm.mtClient != nil {
		if err := rm.mtClient.DeleteRoutingRule(ruleID); err != nil {
			log.Errorf("Failed to delete Mikrotik routing rule: %v", err)
			// 继续执行本地规则删除
		}
	}

	// 解析源 IP
	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return fmt.Errorf("invalid source IP: %s", sourceIP)
	}

	// 创建路由规则（用于匹配）
	rule := netlink.NewRule()
	rule.Src = &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(32, 32),
	}
	rule.Table = tableID

	// 删除规则
	if err := netlink.RuleDel(rule); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	log.Infof("Routing rule deleted successfully")
	return nil
}

// AddTableRoute 为路由表添加默认路由（指向 TUN 设备）
func (rm *RoutingManager) AddTableRoute(tableID int, tunName string) error {
	log.Infof("Adding default route to table %d via %s", tableID, tunName)

	// 等待 TUN 接口就绪
	link, err := rm.WaitForTun(tunName, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to wait for tun: %w", err)
	}

	// 创建默认路由 (0.0.0.0/0)
	_, defaultDst, _ := net.ParseCIDR("0.0.0.0/0")
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       defaultDst,
		Table:     tableID,
	}

	// 先尝试替换（如果已存在）
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("failed to add/replace route: %w", err)
	}

	log.Infof("Default route added to table %d", tableID)
	return nil
}

// DeleteTable 删除路由表的所有路由
func (rm *RoutingManager) DeleteTable(tableID int) error {
	log.Infof("Deleting all routes from table %d", tableID)

	// 获取指定路由表的所有路由
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{
		Table: tableID,
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	// 删除所有路由
	for _, route := range routes {
		if err := netlink.RouteDel(&route); err != nil {
			log.Warnf("Failed to delete route: %v", err)
		}
	}

	log.Infof("All routes deleted from table %d", tableID)
	return nil
}

// WaitForTun 等待 TUN 接口就绪
func (rm *RoutingManager) WaitForTun(tunName string, timeout time.Duration) (netlink.Link, error) {
	log.Infof("Waiting for TUN interface: %s", tunName)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		link, err := netlink.LinkByName(tunName)
		if err == nil {
			// 检查接口是否 UP
			if link.Attrs().Flags&net.FlagUp != 0 {
				log.Infof("TUN interface %s is ready", tunName)
				return link, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for tun %s", tunName)
}

// IsTunUp 检查 TUN 接口是否 UP
func (rm *RoutingManager) IsTunUp(tunName string) bool {
	link, err := netlink.LinkByName(tunName)
	if err != nil {
		return false
	}
	return link.Attrs().Flags&net.FlagUp != 0
}

// GetTunStats 获取 TUN 接口统计信息
func (rm *RoutingManager) GetTunStats(tunName string) (txBytes, rxBytes uint64, err error) {
	link, err := netlink.LinkByName(tunName)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get link: %w", err)
	}

	stats := link.Attrs().Statistics
	if stats == nil {
		return 0, 0, nil
	}

	return stats.TxBytes, stats.RxBytes, nil
}

// ListRules 列出所有路由规则
func (rm *RoutingManager) ListRules() ([]netlink.Rule, error) {
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}
	return rules, nil
}

// InitSystemRouting 初始化系统路由（设置兜底规则，防止跳本地）
func (rm *RoutingManager) InitSystemRouting() error {
	log.Info("Initializing system routing...")

	// 检查是否已存在兜底规则
	rules, err := rm.ListRules()
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	// 检查 priority 32766 和 32767 是否已存在
	hasMainRule := false
	hasDefaultRule := false
	for _, rule := range rules {
		if rule.Priority == 32766 && rule.Table == 254 { // main table
			hasMainRule = true
		}
		if rule.Priority == 32767 && rule.Table == 253 { // default table
			hasDefaultRule = true
		}
	}

	// 如果不存在，添加兜底规则
	if !hasMainRule {
		rule := netlink.NewRule()
		rule.Table = 254 // main table
		rule.Priority = 32766
		if err := netlink.RuleAdd(rule); err != nil {
			log.Warnf("Failed to add main rule (may already exist): %v", err)
		} else {
			log.Info("Added main table rule (priority 32766)")
		}
	}

	if !hasDefaultRule {
		rule := netlink.NewRule()
		rule.Table = 253 // default table
		rule.Priority = 32767
		if err := netlink.RuleAdd(rule); err != nil {
			log.Warnf("Failed to add default rule (may already exist): %v", err)
		} else {
			log.Info("Added default table rule (priority 32767)")
		}
	}

	log.Info("System routing initialized")
	return nil
}
