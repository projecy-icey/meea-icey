package controllers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"meea-icey/models"
	"meea-icey/services"
	"meea-icey/tools"
)

// WechatController 微信消息控制器
type WechatController struct {
	config        *models.Config
	redisClient   *redis.Client
	wechatService *services.WechatService
	ctx           context.Context
}

// NewWechatController 创建微信控制器实例
func NewWechatController(config *models.Config, redisClient *redis.Client, ctx context.Context) *WechatController {
	return &WechatController{
		config:        config,
		redisClient:   redisClient,
		wechatService: services.NewWechatService(config, redisClient, ctx),
		ctx:           ctx,
	}
}

// HandleMessage 处理微信消息请求
func (c *WechatController) HandleMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("接收到微信接口请求，开始处理")
	// 处理GET请求(微信服务器验证)
	if r.Method == "GET" {
		log.Println("接收到微信服务器验证请求")
		signature := r.URL.Query().Get("signature")
		timestamp := r.URL.Query().Get("timestamp")
		nonce := r.URL.Query().Get("nonce")
		echostr := r.URL.Query().Get("echostr")

		log.Printf("验证参数: signature=%s, timestamp=%s, nonce=%s, echostr=%s", signature, timestamp, nonce, echostr)
		// 验证签名
		if tools.VerifyWechatSignature(c.config.Wechat.Token, signature, timestamp, nonce) {
			log.Println("签名验证成功，返回echostr")
			w.Write([]byte(echostr))
			return
		}
		log.Println("签名验证失败")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Invalid signature"))
		return
	}

	// 处理POST请求(接收微信消息)
	log.Println("开始处理微信POST消息请求")
	r.ParseForm()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	log.Printf("接收到微信消息原始数据: %s", string(body))

	// 解析加密消息
	var encryptedMsg tools.EncryptedMessage
	err = xml.Unmarshal(body, &encryptedMsg)
	if err != nil {
		log.Printf("解析加密消息失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 解密消息
	log.Println("开始解密微信消息")
	decryptedMsg, err := tools.DecryptWechatMessage(encryptedMsg.Encrypt, c.config.Wechat.EncodingAESKey, c.config.Wechat.AppID)
	if err != nil {
		log.Printf("解密消息失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("解密后的消息内容: %+v", decryptedMsg)

	// 检查消息类型是否为文本
	if decryptedMsg.MsgType != "text" {
		log.Printf("不支持的消息类型: %s", decryptedMsg.MsgType)
		w.Write([]byte("success"))
		return
	}

	// 提取subject和验证码关键字
	content := decryptedMsg.Content
	codeRegex := regexp.MustCompile(`^(.*?)验证码$`)
	matches := codeRegex.FindStringSubmatch(content)

	if len(matches) < 2 {
		log.Printf("消息内容不包含验证码关键字: %s", content)
		w.Write([]byte("success"))
		return
	}

	subject := strings.TrimSpace(matches[1])
	if subject == "" {
		log.Printf("未提取到主题内容: %s", content)
		w.Write([]byte("success"))
		return
	}

	// 生成六位随机验证码
	code := generateSixDigitCode()
	log.Printf("生成验证码: %s, 主题: %s", code, subject)

	// 计算subject的SHA256哈希
	hash := sha256.Sum256([]byte(subject))
	subjectHash := hex.EncodeToString(hash[:])

	// 构建Redis key
	redisKey := fmt.Sprintf("icey:subject:%s:%s", subjectHash, code)

	// 存储到Redis，初始值为0
	err = c.redisClient.Set(c.ctx, redisKey, 0, 24*time.Hour).Err()
	if err != nil {
		log.Printf("存储验证码到Redis失败: %v", err)
		w.Write([]byte("success"))
		return
	}
	log.Printf("验证码成功存储到Redis: %s", redisKey)

	// 构建被动回复XML
	msgContent := fmt.Sprintf("您的验证码是: %s", code)
	replyXML := fmt.Sprintf(`<xml>
  <ToUserName><![CDATA[%s]]></ToUserName>
  <FromUserName><![CDATA[%s]]></FromUserName>
  <CreateTime>%d</CreateTime>
  <MsgType><![CDATA[text]]></MsgType>
  <Content><![CDATA[%s]]></Content>
</xml>`, decryptedMsg.FromUserName, decryptedMsg.ToUserName, time.Now().Unix(), msgContent)

	w.Header().Set("Content-Type", "application/xml")
	if _, err := w.Write([]byte(replyXML)); err != nil {
		log.Printf("发送被动回复失败: %v", err)
	}
}

// 生成六位随机数字验证码
func generateSixDigitCode() string {
	max := big.NewInt(900000)
	min := big.NewInt(100000)
	// 生成 [0, 900000) 之间的随机数
	randNum, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "123456"
	}
	// 加上 min，得到 [100000, 1000000) 之间的随机数
	result := new(big.Int).Add(randNum, min)
	return result.String()
}