package singbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// ProcessManager sing-box 进程管理器
type ProcessManager struct {
	singboxBin string
	configDir  string
	logDir     string
}

// NewProcessManager 创建进程管理器
func NewProcessManager(singboxBin, configDir, logDir string) *ProcessManager {
	return &ProcessManager{
		singboxBin: singboxBin,
		configDir:  configDir,
		logDir:     logDir,
	}
}

// StartProcess 启动 sing-box 进程（独立于 Manager）
// 关键：使用 syscall.Setsid 让进程成为新的会话组长，独立于 Manager
func (pm *ProcessManager) StartProcess(nodeID int, configPath string) (int, error) {
	// 确保配置文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("config file not found: %s", configPath)
	}

	// 创建日志文件
	logPath := filepath.Join(pm.logDir, fmt.Sprintf("node_%d.log", nodeID))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// 创建命令
	cmd := exec.Command(pm.singboxBin, "run", "-c", configPath)

	// 设置输出到日志文件
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 关键：设置进程属性，使其独立于父进程
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // 创建新的会话，进程成为会话组长
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start sing-box: %w", err)
	}

	pid := cmd.Process.Pid
	log.Infof("Started sing-box process for node %d, PID: %d", nodeID, pid)

	// 注意：不调用 cmd.Wait()，让进程独立运行
	// Manager 终止时不会影响这个进程

	return pid, nil
}

// StopProcess 停止 sing-box 进程
func (pm *ProcessManager) StopProcess(pid int) error {
	if pid == 0 {
		return fmt.Errorf("invalid pid: 0")
	}

	// 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// 发送 SIGTERM 信号优雅停止
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// 如果 SIGTERM 失败，尝试 SIGKILL
		log.Warnf("SIGTERM failed, trying SIGKILL for PID %d", pid)
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	log.Infof("Stopped sing-box process, PID: %d", pid)
	return nil
}

// IsProcessRunning 检查进程是否在运行
func (pm *ProcessManager) IsProcessRunning(pid int) bool {
	if pid == 0 {
		return false
	}

	// 尝试向进程发送信号 0（不实际发送，只检查进程是否存在）
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号 0 检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	return true
}

// GetProcessCmdline 获取进程的命令行（用于验证是否是 sing-box 进程）
func (pm *ProcessManager) GetProcessCmdline(pid int) (string, error) {
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return "", err
	}

	// cmdline 以 null 分隔，转换为空格
	cmdline := string(data)
	for i, c := range cmdline {
		if c == 0 {
			cmdline = cmdline[:i] + " " + cmdline[i+1:]
		}
	}

	return cmdline, nil
}

// VerifySingboxProcess 验证 PID 是否是 sing-box 进程
func (pm *ProcessManager) VerifySingboxProcess(pid int) bool {
	if !pm.IsProcessRunning(pid) {
		return false
	}

	cmdline, err := pm.GetProcessCmdline(pid)
	if err != nil {
		return false
	}

	// 检查命令行是否包含 sing-box
	return len(cmdline) > 0 && (cmdline[0:8] == "sing-box" || filepath.Base(cmdline) == "sing-box")
}
