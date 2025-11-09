package handlers

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/firadio/golang-singbox-manager/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// LogHandler 日志处理器
type LogHandler struct {
	logDir string
}

// NewLogHandler 创建日志处理器
func NewLogHandler(logDir string) *LogHandler {
	return &LogHandler{
		logDir: logDir,
	}
}

// GetNodeLogs 获取节点日志
func (h *LogHandler) GetNodeLogs(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2001, "Invalid node ID")
		return
	}

	// 验证节点是否存在
	node, err := storage.GetNode(nodeID)
	if err != nil {
		Error(c, 2002, "Node not found")
		return
	}

	// 获取查询参数
	linesParam := c.DefaultQuery("lines", "100")
	lines, err := strconv.Atoi(linesParam)
	if err != nil || lines < 1 {
		lines = 100
	}
	if lines > 10000 {
		lines = 10000 // 最多返回 10000 行
	}

	// 构建日志文件路径
	logFile := h.logDir + "/node_" + strconv.Itoa(nodeID) + ".log"

	// 检查日志文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		Success(c, gin.H{
			"node_id": nodeID,
			"logs":    []string{},
			"message": "Log file not found (node may not have been started yet)",
		})
		return
	}

	// 读取日志文件的最后 N 行
	logLines, err := h.tailFile(logFile, lines)
	if err != nil {
		log.Errorf("Failed to read log file: %v", err)
		Error(c, 2003, "Failed to read log file")
		return
	}

	Success(c, gin.H{
		"node_id":   nodeID,
		"node_name": node.Name,
		"logs":      logLines,
		"lines":     len(logLines),
	})
}

// tailFile 读取文件的最后 N 行
func (h *LogHandler) tailFile(filename string, n int) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	// 如果文件很小，直接读取所有内容
	if fileSize < 1024*100 { // 小于 100KB
		return h.readAllLines(file, n)
	}

	// 对于大文件，从文件末尾开始读取
	return h.tailLargeFile(file, fileSize, n)
}

// readAllLines 读取所有行并返回最后 N 行
func (h *LogHandler) readAllLines(file *os.File, n int) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(file)

	// 设置更大的缓冲区，处理长行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 返回最后 N 行
	if len(lines) > n {
		return lines[len(lines)-n:], nil
	}
	return lines, nil
}

// tailLargeFile 从大文件末尾读取最后 N 行
func (h *LogHandler) tailLargeFile(file *os.File, fileSize int64, n int) ([]string, error) {
	const avgLineSize = 200 // 假设平均每行 200 字节
	bufferSize := int64(n * avgLineSize * 2)

	// 确保缓冲区不超过文件大小
	if bufferSize > fileSize {
		bufferSize = fileSize
	}

	// 从文件末尾开始读取
	startPos := fileSize - bufferSize
	if startPos < 0 {
		startPos = 0
	}

	_, err := file.Seek(startPos, io.SeekStart)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string

	// 如果不是从文件开始读取，跳过第一行（可能不完整）
	if startPos > 0 {
		scanner.Scan()
	}

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 返回最后 N 行
	if len(lines) > n {
		return lines[len(lines)-n:], nil
	}
	return lines, nil
}

// ClearNodeLogs 清空节点日志
func (h *LogHandler) ClearNodeLogs(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2001, "Invalid node ID")
		return
	}

	// 验证节点是否存在
	_, err = storage.GetNode(nodeID)
	if err != nil {
		Error(c, 2002, "Node not found")
		return
	}

	// 构建日志文件路径
	logFile := h.logDir + "/node_" + strconv.Itoa(nodeID) + ".log"

	// 检查日志文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		Success(c, gin.H{"message": "Log file does not exist"})
		return
	}

	// 清空日志文件
	err = os.Truncate(logFile, 0)
	if err != nil {
		log.Errorf("Failed to clear log file: %v", err)
		Error(c, 2004, "Failed to clear log file")
		return
	}

	log.Infof("Cleared logs for node %d", nodeID)
	Success(c, gin.H{"message": "Log file cleared successfully"})
}

// SearchNodeLogs 搜索节点日志
func (h *LogHandler) SearchNodeLogs(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, 2001, "Invalid node ID")
		return
	}

	// 验证节点是否存在
	node, err := storage.GetNode(nodeID)
	if err != nil {
		Error(c, 2002, "Node not found")
		return
	}

	// 获取搜索关键词
	keyword := c.Query("keyword")
	if keyword == "" {
		Error(c, 2005, "Search keyword is required")
		return
	}

	// 获取最大返回行数
	maxLinesParam := c.DefaultQuery("max_lines", "1000")
	maxLines, err := strconv.Atoi(maxLinesParam)
	if err != nil || maxLines < 1 {
		maxLines = 1000
	}
	if maxLines > 10000 {
		maxLines = 10000
	}

	// 构建日志文件路径
	logFile := h.logDir + "/node_" + strconv.Itoa(nodeID) + ".log"

	// 检查日志文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		Success(c, gin.H{
			"node_id": nodeID,
			"logs":    []string{},
			"message": "Log file not found",
		})
		return
	}

	// 搜索日志
	matchedLines, err := h.searchFile(logFile, keyword, maxLines)
	if err != nil {
		log.Errorf("Failed to search log file: %v", err)
		Error(c, 2006, "Failed to search log file")
		return
	}

	Success(c, gin.H{
		"node_id":   nodeID,
		"node_name": node.Name,
		"keyword":   keyword,
		"logs":      matchedLines,
		"count":     len(matchedLines),
	})
}

// searchFile 在文件中搜索包含关键词的行
func (h *LogHandler) searchFile(filename, keyword string, maxLines int) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matchedLines []string
	scanner := bufio.NewScanner(file)

	// 设置更大的缓冲区
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	keywordLower := strings.ToLower(keyword)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), keywordLower) {
			matchedLines = append(matchedLines, line)
			if len(matchedLines) >= maxLines {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return matchedLines, nil
}
