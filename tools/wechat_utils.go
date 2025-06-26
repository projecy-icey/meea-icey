package tools

import (
	"encoding/binary"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
)

// EncryptedMessage 微信加密消息结构体
type EncryptedMessage struct {
	XMLName     xml.Name `xml:"xml"`
	ToUserName  string   `xml:"ToUserName"`
	Encrypt     string   `xml:"Encrypt"`
}

// DecryptedMessage 解密后的微信消息结构体
type DecryptedMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
}

// PKCS7Unpad 移除PKCS7填充
func PKCS7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("invalid padding")
	}
	unpadding := int(data[length-1])
	if unpadding > length || unpadding == 0 {
		return nil, errors.New("invalid padding")
	}
	// 验证所有填充字节是否正确
	for i := length - unpadding; i < length; i++ {
		if data[i] != byte(unpadding) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:(length - unpadding)], nil
}

// min 返回两个整数中的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DecryptWechatMessage 解密微信消息
func DecryptWechatMessage(encryptedData string, encodingAESKey string, expectedAppID string) (*DecryptedMessage, error) {
	// 解码EncodingAESKey
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("解码EncodingAESKey失败: %v", err)
	}

	// 验证AES密钥长度
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("无效的EncodingAESKey长度: %d, 预期32字节", len(aesKey))
	}

	// 创建AES解密器
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES解密器失败: %v", err)
	}

	// 解码加密数据
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("解码加密数据失败: %v", err)
	}

	// 检查密文长度是否为块大小的倍数
	blockSize := block.BlockSize()
	if len(ciphertext) % blockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小的倍数: %d", len(ciphertext))
	}

	// 微信规范：使用密钥前16字节作为IV
	if len(aesKey) < 16 {
		return nil, errors.New("AES密钥长度不足IV长度")
	}
	iv := aesKey[:16]

	stream := cipher.NewCBCDecrypter(block, iv)

	// 解密
	stream.CryptBlocks(ciphertext, ciphertext)

	// 移除PKCS7填充
	plaintext, err := PKCS7Unpad(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("移除PKCS7填充失败: %v", err)
	}

	// 解析明文结构 (16字节随机数 + 4字节消息长度 + 消息内容 + appid)
	log.Printf("解密后明文长度: %d, 完整明文: %x", len(plaintext), plaintext)
	if len(plaintext) < 20 {
		return nil, errors.New("解密后数据太短")
	}

	// 提取16字节随机数
	random := plaintext[:16]
	log.Printf("随机数: %x", random)

	// 从16字节随机数后提取4字节消息长度（网络字节序）
	lengthBytes := plaintext[16:20]
	log.Printf("消息长度字段原始字节: %x", lengthBytes)

	// 微信规范：消息长度为4字节网络字节序（大端序）整数
	// 验证长度字段是否为有效的二进制整数
	if lengthBytes[0] == 0x3c { // '<'字符，表明可能解析位置错误
		return nil, fmt.Errorf("消息长度字段位置错误，可能是XML内容: %x", lengthBytes)
	}
	log.Printf("消息长度字段原始字节: %x", lengthBytes)
	length := binary.BigEndian.Uint32(lengthBytes)
	msgLen := int(length)
	log.Printf("解析出的消息长度: %d", msgLen)

	// 验证消息长度是否有效
	if msgLen <= 0 || msgLen > len(plaintext)-20 {
		return nil, fmt.Errorf("无效的消息长度: %d, 实际可用长度: %d", msgLen, len(plaintext)-20)
	}

	// 提取消息内容和AppID
	contentEnd := 20 + msgLen
	if contentEnd > len(plaintext) {
		return nil, errors.New("消息内容超出明文长度")
	}
	msgContent := plaintext[20:contentEnd]
	appID := string(plaintext[contentEnd:])
	log.Printf("解密得到AppID: %s", appID)

	// 验证AppID（需从配置传入正确AppID）
	if appID != expectedAppID {
		return nil, fmt.Errorf("AppID验证失败: 预期%s, 实际%s", expectedAppID, appID)
	}

	// 解析XML消息
	var msg DecryptedMessage
	err = xml.Unmarshal(msgContent, &msg)
	if err != nil {
		return nil, fmt.Errorf("解析XML消息失败: %v, 消息内容: %s", err, string(msgContent))
	}

	return &msg, nil
}

func VerifyWechatSignature(token, signature, timestamp, nonce string) bool {
	if token == "" || signature == "" || timestamp == "" || nonce == "" {
		return false
	}
	// 将token、timestamp、nonce按字典序排序
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)
	// 拼接并计算SHA1
	sha1Hash := sha1.New()
	io.WriteString(sha1Hash, strings.Join(strs, ""))
	computedSignature := hex.EncodeToString(sha1Hash.Sum(nil))
	// 比较签名
	return computedSignature == signature
}