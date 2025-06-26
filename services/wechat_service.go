package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"meea-icey/models"
)

// WechatService 微信服务
type WechatService struct {
	config     *models.Config
	redisClient *redis.Client
	ctx        context.Context
}

// NewWechatService 创建微信服务实例
func NewWechatService(config *models.Config, redisClient *redis.Client, ctx context.Context) *WechatService {
	return &WechatService{
		config:     config,
		redisClient: redisClient,
		ctx:        ctx,
	}
}

// AccessToken 微信access_token结构体
type AccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// GetAccessToken 获取微信access_token
func (s *WechatService) GetAccessToken() (string, error) {
	// 尝试从Redis获取缓存的access_token
	accessToken, err := s.redisClient.Get(s.ctx, "wechat_access_token").Result()
	if err == nil {
		if accessToken != "" {
			log.Printf("从Redis获取access_token成功: %s", accessToken)
			return accessToken, nil
		}
		log.Printf("Redis中的access_token为空字符串")
	} else if err != redis.Nil {
		log.Printf("从Redis获取access_token失败: %v", err)
	}

	// Redis中没有缓存或已过期，调用微信接口获取
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", s.config.Wechat.AppID, s.config.Wechat.AppSecret)
	log.Printf("请求access_token URL: %s", url)

	// 创建HTTP客户端，不使用代理
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("获取access_token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取access_token响应失败: %v", err)
	}
	log.Printf("微信access_token响应: %s", string(body))

	var result AccessToken
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("解析access_token响应失败: %v, 响应内容: %s", err, string(body))
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("微信接口返回错误: errcode=%d, errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	if result.AccessToken == "" {
		return "", errors.New("微信接口返回空的access_token")
	}

	// 将access_token存入Redis，并设置过期时间
	expireDuration := time.Duration(result.ExpiresIn-300) * time.Second // 提前300秒过期
	err = s.redisClient.Set(s.ctx, "wechat_access_token", result.AccessToken, expireDuration).Err()
	if err != nil {
		log.Printf("缓存access_token到Redis失败: %v", err)
		// 不中断程序，继续返回access_token
	}

	log.Printf("获取access_token成功: %s", result.AccessToken)
	return result.AccessToken, nil
}

// CustomerMessage 微信客服消息结构体
type CustomerMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

// SendMessage 发送微信客服消息
func (s *WechatService) SendMessage(openid, content string) error {
	accessToken, err := s.GetAccessToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s", accessToken)

	msg := CustomerMessage{
		ToUser:  openid,
		MsgType: "text",
	}
	msg.Text.Content = content

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("构造消息JSON失败: %v", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取消息发送响应失败: %v", err)
	}

	// 检查微信接口返回结果
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析消息发送响应失败: %v, 响应内容: %s", err, string(body))
	}

	if result["errcode"].(float64) != 0 {
		return fmt.Errorf("微信接口返回错误: 错误码=%v, 错误信息=%v, 响应内容=%s",
			result["errcode"], result["errmsg"], string(body))
	}

	return nil
}