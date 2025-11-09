package singbox

import (
	"fmt"
	"time"

	"github.com/firadio/golang-singbox-manager/internal/storage"
	log "github.com/sirupsen/logrus"
)

// Manager sing-box 管理器
type Manager struct {
	pm        *ProcessManager
	configDir string
}

// NewManager 创建 Manager
func NewManager(singboxBin, configDir, logDir string) *Manager {
	return &Manager{
		pm:        NewProcessManager(singboxBin, configDir, logDir),
		configDir: configDir,
	}
}

// StartNode 启动节点
func (m *Manager) StartNode(node *storage.ProxyNode) error {
	log.Infof("Starting node: %s (ID: %d)", node.Name, node.ID)

	// 检查节点是否已经在运行
	if node.Pid > 0 && m.pm.IsProcessRunning(node.Pid) {
		log.Warnf("Node %s is already running (PID: %d)", node.Name, node.Pid)
		return nil
	}

	// 获取 detour 节点（如果有）
	var detourNode *storage.ProxyNode
	if node.DetourID != nil {
		var err error
		detourNode, err = storage.GetNode(*node.DetourID)
		if err != nil {
			return fmt.Errorf("failed to get detour node: %w", err)
		}
	}

	// 生成配置文件
	configContent, err := GenerateConfig(node, detourNode)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// 保存配置文件
	configPath, err := SaveConfig(m.configDir, node.ID, configContent)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	log.Infof("Config saved: %s", configPath)

	// 更新状态为 starting
	if err := storage.UpdateNodeStatus(node.ID, "starting", 0); err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	// 启动 sing-box 进程
	pid, err := m.pm.StartProcess(node.ID, configPath)
	if err != nil {
		storage.UpdateNodeStatus(node.ID, "error", 0)
		return fmt.Errorf("failed to start process: %w", err)
	}

	// 等待进程启动
	time.Sleep(1 * time.Second)

	// 验证进程是否在运行
	if !m.pm.IsProcessRunning(pid) {
		storage.UpdateNodeStatus(node.ID, "error", 0)
		return fmt.Errorf("process exited unexpectedly")
	}

	// 更新状态为 running
	if err := storage.UpdateNodeStatus(node.ID, "running", pid); err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	log.Infof("Node %s started successfully (PID: %d)", node.Name, pid)
	return nil
}

// StopNode 停止节点
func (m *Manager) StopNode(nodeID int) error {
	log.Infof("Stopping node ID: %d", nodeID)

	// 获取节点信息
	node, err := storage.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// 检查节点是否在运行
	if node.Pid == 0 {
		log.Warnf("Node %s is not running", node.Name)
		return nil
	}

	// 停止进程
	if err := m.pm.StopProcess(node.Pid); err != nil {
		log.Errorf("Failed to stop process (PID: %d): %v", node.Pid, err)
		// 继续执行，更新状态
	}

	// 更新状态为 stopped
	if err := storage.UpdateNodeStatus(nodeID, "stopped", 0); err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	log.Infof("Node %s stopped", node.Name)
	return nil
}

// RestartNode 重启节点
func (m *Manager) RestartNode(nodeID int) error {
	log.Infof("Restarting node ID: %d", nodeID)

	// 先停止
	if err := m.StopNode(nodeID); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	// 等待进程完全停止
	time.Sleep(2 * time.Second)

	// 重新获取节点信息
	node, err := storage.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// 再启动
	if err := m.StartNode(node); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	return nil
}

// GetNodeStatus 获取节点状态
func (m *Manager) GetNodeStatus(nodeID int) (string, error) {
	node, err := storage.GetNode(nodeID)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %w", err)
	}

	// 检查实际进程状态
	if node.Pid > 0 {
		if m.pm.IsProcessRunning(node.Pid) {
			// 进程在运行，但数据库状态可能不一致
			if node.Status != "running" {
				storage.UpdateNodeStatus(nodeID, "running", node.Pid)
				return "running", nil
			}
			return node.Status, nil
		} else {
			// 进程已停止，但数据库状态可能不一致
			if node.Status != "stopped" && node.Status != "error" {
				storage.UpdateNodeStatus(nodeID, "error", 0)
				return "error", nil
			}
		}
	}

	return node.Status, nil
}

// RecoverNodes 恢复节点（Manager 重启后调用）
func (m *Manager) RecoverNodes() error {
	log.Info("Recovering nodes from database...")

	nodes, err := storage.GetEnabledNodes()
	if err != nil {
		return fmt.Errorf("failed to get enabled nodes: %w", err)
	}

	for _, node := range nodes {
		// 检查数据库中记录的 PID 是否还在运行
		if node.Pid > 0 && m.pm.IsProcessRunning(node.Pid) {
			log.Infof("Node %s is already running (PID: %d), reattaching", node.Name, node.Pid)
			// 进程还在运行，只需要更新状态
			storage.UpdateNodeStatus(node.ID, "running", node.Pid)
			continue
		}

		// 进程不在运行，重新启动
		log.Infof("Node %s is not running, restarting", node.Name)
		if err := m.StartNode(node); err != nil {
			log.Errorf("Failed to start node %s: %v", node.Name, err)
			storage.UpdateNodeStatus(node.ID, "error", 0)
			continue
		}
	}

	log.Info("Node recovery completed")
	return nil
}
