package mikrotik

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/firadio/golang-singbox-manager/internal/config"
	log "github.com/sirupsen/logrus"
)

// Client Mikrotik客户端
type Client struct {
	config *config.MikrotikConfig
	conn   net.Conn
	reader *bufio.Reader
}

// NewClient 创建Mikrotik客户端
func NewClient(cfg *config.MikrotikConfig) *Client {
	return &Client{
		config: cfg,
	}
}

// Connect 连接到Mikrotik
func (c *Client) Connect() error {
	if !c.config.Enabled {
		return nil
	}

	address := fmt.Sprintf("%s:%d", c.config.Address, c.config.Port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to Mikrotik: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)

	// 登录认证
	if err := c.login(); err != nil {
		c.Close()
		return fmt.Errorf("failed to login: %w", err)
	}

	log.Infof("Connected to Mikrotik at %s", address)
	return nil
}

// Close 关闭连接
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// login 执行登录认证
func (c *Client) login() error {
	log.Debugf("Mikrotik: Attempting login to %s:%d as user %s", c.config.Address, c.config.Port, c.config.Username)

	// 先尝试新版认证方式（RouterOS 6.43+）- 直接发送用户名和密码
	log.Debug("Mikrotik: Trying new-style authentication (RouterOS 6.43+)")
	attrs := map[string]string{
		"name":     c.config.Username,
		"password": c.config.Password,
	}
	if err := c.writeCommand("/login", attrs); err != nil {
		log.Errorf("Mikrotik: Failed to send login command: %v", err)
		return err
	}

	reply, err := c.readReply()
	if err != nil {
		log.Errorf("Mikrotik: Failed to read login response: %v", err)
		return err
	}

	// 检查新版认证是否成功
	for _, sentence := range reply {
		if len(sentence) > 0 && sentence[0] == "!done" {
			log.Debug("Mikrotik: New-style authentication successful")
			return nil
		}
		// 如果返回challenge，说明需要用旧版认证
		if len(sentence) > 0 && sentence[0] == "!done" {
			for _, word := range sentence {
				if strings.HasPrefix(word, "=ret=") {
					// 有challenge，使用旧版认证
					challengeHex := strings.TrimPrefix(word, "=ret=")
					log.Debug("Mikrotik: Server requires old-style authentication, received challenge")
					return c.loginOldStyle(challengeHex)
				}
			}
		}
		if len(sentence) > 0 && sentence[0] == "!trap" {
			// 提取错误信息
			errorMsg := "login failed"
			for _, word := range sentence {
				if strings.HasPrefix(word, "=message=") {
					errorMsg = strings.TrimPrefix(word, "=message=")
					break
				}
			}
			return fmt.Errorf("%s", errorMsg)
		}
	}

	return nil
}

// loginOldStyle 旧版MD5 challenge-response认证（RouterOS < 6.43）
func (c *Client) loginOldStyle(challengeHex string) error {
	log.Debugf("Mikrotik: Using old-style authentication with challenge: %s", challengeHex)

	// 解码十六进制challenge
	challengeBytes, err := hex.DecodeString(challengeHex)
	if err != nil {
		return fmt.Errorf("failed to decode challenge: %w", err)
	}
	log.Debugf("Mikrotik: Decoded challenge bytes (len=%d)", len(challengeBytes))

	// 计算MD5响应
	h := md5.New()
	h.Write([]byte{0})
	h.Write([]byte(c.config.Password))
	h.Write(challengeBytes)
	response := hex.EncodeToString(h.Sum(nil))

	log.Debugf("Mikrotik: Computed response hash: %s", response)

	// 发送认证
	attrs := map[string]string{
		"name":     c.config.Username,
		"response": "00" + response,
	}
	if err := c.writeCommand("/login", attrs); err != nil {
		return err
	}

	// 读取登录结果
	reply, err := c.readReply()
	if err != nil {
		return err
	}

	// 检查是否成功
	for _, sentence := range reply {
		if len(sentence) > 0 && sentence[0] == "!done" {
			log.Debug("Mikrotik: Old-style authentication successful")
			return nil
		}
		if len(sentence) > 0 && sentence[0] == "!trap" {
			// 提取错误信息
			errorMsg := "login failed"
			for _, word := range sentence {
				if strings.HasPrefix(word, "=message=") {
					errorMsg = strings.TrimPrefix(word, "=message=")
					break
				}
			}
			return fmt.Errorf("%s", errorMsg)
		}
	}

	return nil
}

// writeCommand 写入命令
func (c *Client) writeCommand(command string, attrs map[string]string) error {
	// 写入命令
	if err := c.writeWord(command); err != nil {
		return err
	}

	// 写入属性
	for key, value := range attrs {
		attr := fmt.Sprintf("=%s=%s", key, value)
		if err := c.writeWord(attr); err != nil {
			return err
		}
	}

	// 写入空字节标记命令结束
	if err := c.writeByte(0); err != nil {
		return err
	}

	return nil
}

// writeWord 写入一个词
func (c *Client) writeWord(word string) error {
	// 写入长度
	if err := c.writeLen(len(word)); err != nil {
		return err
	}

	// 写入内容
	_, err := c.conn.Write([]byte(word))
	return err
}

// writeLen 写入长度
func (c *Client) writeLen(l int) error {
	if l < 0x80 {
		return c.writeByte(byte(l))
	} else if l < 0x4000 {
		return c.writeBytes([]byte{byte(l>>8) | 0x80, byte(l)})
	} else if l < 0x200000 {
		return c.writeBytes([]byte{byte(l>>16) | 0xC0, byte(l >> 8), byte(l)})
	} else if l < 0x10000000 {
		return c.writeBytes([]byte{byte(l>>24) | 0xE0, byte(l >> 16), byte(l >> 8), byte(l)})
	}
	return c.writeBytes([]byte{0xF0, byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)})
}

// writeByte 写入单字节
func (c *Client) writeByte(b byte) error {
	_, err := c.conn.Write([]byte{b})
	return err
}

// writeBytes 写入字节数组
func (c *Client) writeBytes(b []byte) error {
	_, err := c.conn.Write(b)
	return err
}

// readReply 读取响应
func (c *Client) readReply() ([][]string, error) {
	var reply [][]string

	for {
		sentence, err := c.readSentence()
		if err != nil {
			return nil, err
		}

		if len(sentence) == 0 {
			break
		}

		reply = append(reply, sentence)

		// !done 表示命令完成
		if len(sentence) > 0 && sentence[0] == "!done" {
			break
		}
	}

	return reply, nil
}

// readSentence 读取一个句子
func (c *Client) readSentence() ([]string, error) {
	var sentence []string

	for {
		word, err := c.readWord()
		if err != nil {
			return nil, err
		}

		if word == "" {
			break
		}

		sentence = append(sentence, word)
	}

	return sentence, nil
}

// readWord 读取一个词
func (c *Client) readWord() (string, error) {
	// 读取长度
	length, err := c.readLen()
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	// 读取内容
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.reader, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// readLen 读取长度
func (c *Client) readLen() (int, error) {
	b, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}

	if b&0x80 == 0 {
		return int(b), nil
	} else if b&0xC0 == 0x80 {
		b2, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		return int(b&^0x80)<<8 | int(b2), nil
	} else if b&0xE0 == 0xC0 {
		b2, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		b3, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		return int(b&^0xC0)<<16 | int(b2)<<8 | int(b3), nil
	} else if b&0xF0 == 0xE0 {
		b2, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		b3, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		b4, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		return int(b&^0xE0)<<24 | int(b2)<<16 | int(b3)<<8 | int(b4), nil
	}

	// 5-byte length
	b2, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	b5, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	return int(b2)<<24 | int(b3)<<16 | int(b4)<<8 | int(b5), nil
}

// runCommand 执行命令
func (c *Client) runCommand(command string, attrs map[string]string) ([]map[string]string, error) {
	if !c.config.Enabled || c.conn == nil {
		return nil, nil
	}

	if err := c.writeCommand(command, attrs); err != nil {
		return nil, err
	}

	reply, err := c.readReply()
	if err != nil {
		return nil, err
	}

	var result []map[string]string
	for _, sentence := range reply {
		item := make(map[string]string)
		for _, word := range sentence {
			if strings.HasPrefix(word, "=") {
				parts := strings.SplitN(word[1:], "=", 2)
				if len(parts) == 2 {
					item[parts[0]] = parts[1]
				}
			}
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}

	return result, nil
}

// AddRoutingRule 添加路由规则
func (c *Client) AddRoutingRule(sourceIP string, ruleID int) error {
	if !c.config.Enabled {
		return nil
	}

	// 构建comment用于标识规则
	comment := fmt.Sprintf("sing-box-rule-id-%d", ruleID)

	// 先检查规则是否已存在 - 获取所有规则并在本地过滤
	items, err := c.runCommand("/routing/rule/print", nil)
	if err != nil {
		log.Warnf("Failed to list Mikrotik rules: %v", err)
		// 继续尝试添加
	} else {
		// 在返回的规则中查找匹配的 comment
		for _, item := range items {
			if itemComment, ok := item["comment"]; ok && itemComment == comment {
				log.Infof("Mikrotik routing rule already exists: %s (comment=%s), skipping", sourceIP, comment)
				return nil
			}
		}
	}

	// 规则不存在，添加新规则
	attrs := map[string]string{
		"action":       "lookup-only-in-table",
		"src-address":  sourceIP + "/32",
		"table":        c.config.RoutingTable,
		"place-before": "1",
		"comment":      comment,
	}

	_, err = c.runCommand("/routing/rule/add", attrs)
	if err != nil {
		return fmt.Errorf("failed to add routing rule: %w", err)
	}

	log.Infof("Added Mikrotik routing rule: %s -> %s (comment=%s)", sourceIP, c.config.RoutingTable, comment)
	return nil
}

// DeleteRoutingRule 删除路由规则
func (c *Client) DeleteRoutingRule(ruleID int) error {
	if !c.config.Enabled {
		return nil
	}

	// 构建comment用于查找规则
	comment := fmt.Sprintf("sing-box-rule-id-%d", ruleID)

	// 先获取所有规则
	items, err := c.runCommand("/routing/rule/print", nil)
	if err != nil {
		log.Errorf("Failed to list Mikrotik rules: %v", err)
		return fmt.Errorf("failed to list routing rules: %w", err)
	}

	log.Debugf("Mikrotik print returned %d total rules", len(items))

	// 在规则中查找匹配 comment 的规则
	var matchedItems []map[string]string
	for _, item := range items {
		if itemComment, ok := item["comment"]; ok && itemComment == comment {
			matchedItems = append(matchedItems, item)
			log.Debugf("Found matching rule: %+v", item)
		}
	}

	if len(matchedItems) == 0 {
		log.Infof("Mikrotik routing rule not found: comment=%s, already deleted", comment)
		return nil
	}

	// 删除找到的所有匹配规则
	deletedCount := 0
	for _, item := range matchedItems {
		id, hasID := item[".id"]
		log.Debugf("Processing item: hasID=%v, id=%s", hasID, id)

		if hasID && id != "" {
			log.Infof("Attempting to remove Mikrotik rule with id=%s", id)
			_, err := c.runCommand("/routing/rule/remove", map[string]string{
				".id": id,
			})
			if err != nil {
				log.Errorf("Failed to remove Mikrotik rule %s: %v", id, err)
			} else {
				log.Infof("Deleted Mikrotik routing rule: comment=%s, id=%s", comment, id)
				deletedCount++
			}
		} else {
			log.Warnf("Item has no .id field or id is empty: %+v", item)
		}
	}

	if deletedCount == 0 {
		return fmt.Errorf("failed to delete any routing rules")
	}

	return nil
}

// TestConnection 测试连接
func (c *Client) TestConnection() error {
	if !c.config.Enabled {
		return fmt.Errorf("Mikrotik is not enabled")
	}

	address := fmt.Sprintf("%s:%d", c.config.Address, c.config.Port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	tempClient := &Client{
		config: c.config,
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	if err := tempClient.login(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	return nil
}
