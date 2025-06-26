package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"golang.org/x/crypto/bcrypt"
)

// 全局雪花ID生成器
var snowflakeNode *snowflake.Node

// 初始化雪花ID生成器
func init() {
	node, err := snowflake.NewNode(1)
	if err != nil {
		panic("初始化雪花ID生成器失败: " + err.Error())
	}
	snowflakeNode = node
}

// GenerateSnowflakeID 生成雪花ID
func GenerateSnowflakeID() (string, error) {
	id := snowflakeNode.Generate()
	return id.String(), nil
}

// GenerateRandomToken 生成随机token
func GenerateRandomToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("生成随机token失败: " + err.Error())
	}
	token := base64.URLEncoding.EncodeToString(b)
	return strings.ReplaceAll(token, "-", "2")
}

// BuildSubjectPath 构建基于subject的目录路径
func BuildSubjectPath(basePath, subject string) (string, string) {
	if len(subject) < 6 {
		return "", ""
	}
	
	part1 := subject[:2]
	part2 := subject[2:4]
	part3 := subject[4:6]
	
	// 完整路径
	fullPath := filepath.Join(basePath, "icey-storage", part1, part2, part3, subject)
	// 相对路径
	relativePath := filepath.Join(part1, part2, part3, subject)
	
	return fullPath, relativePath
}

// GenerateFileNamePrefix 生成文件名前缀 (时间戳-雪花ID)
func GenerateFileNamePrefix() (string, error) {
	timestamp := time.Now().UnixMilli()
	id, err := GenerateSnowflakeID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s", timestamp, id), nil
}

// CreateFileWithContent 创建文件并写入内容
func CreateFileWithContent(filePath string, content []byte, perm os.FileMode) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	
	return os.WriteFile(filePath, content, perm)
}

// CreateTokenFile 创建token文件
func CreateTokenFile(filePath, subject, token string) error {
	hash := sha256.Sum256([]byte(subject + token))
	hashedToken, err := bcrypt.GenerateFromPassword(hash[:], bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成token失败: %v", err)
	}
	
	return CreateFileWithContent(filePath, hashedToken, 0644)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileSize 获取文件大小
func GetFileSize(path string) int64 {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

// Min 返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseTokenAndID 解析token和ID
func ParseTokenAndID(fullToken string) (string, string, error) {
	lastDashIndex := strings.LastIndex(fullToken, "-")
	if lastDashIndex == -1 || lastDashIndex >= len(fullToken)-1 {
		return "", "", fmt.Errorf("无效的token格式")
	}
	
	token := fullToken[:lastDashIndex]
	id := fullToken[lastDashIndex+1:]
	return token, id, nil
}

// ValidateToken 验证token
func ValidateToken(hashedToken []byte, subject, token string) error {
	hash := sha256.Sum256([]byte(subject + token))
	return bcrypt.CompareHashAndPassword(hashedToken, hash[:])
} 