package services

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"meea-icey/models"
)

type VerifyService struct {
	redisClient *redis.Client
	config      *models.Config
}

func NewVerifyService(redisClient *redis.Client, config *models.Config) *VerifyService {
	return &VerifyService{
		redisClient: redisClient,
		config:      config,
	}
}

func (s *VerifyService) VerifyCode(subject, code string) (bool, error) {
	// 拼接Redis Key
	redisKey := fmt.Sprintf("icey:subject:%s:%s", subject, code)
	fmt.Printf("Verifying code with Redis key: %s\n", redisKey)

	// 检查Key是否存在
	exists, err := s.redisClient.Exists(context.Background(), redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("检查验证码失败: %v", err)
	}
	if exists == 0 {
		return false, nil
	}

	// 检查使用次数
	count, err := s.redisClient.Get(context.Background(), redisKey).Int()
	if err != nil {
		return false, fmt.Errorf("获取验证码使用次数失败: %v", err)
	}
	if count >= s.config.Verification.MaxAttempts {
		// 删除超过使用次数限制的验证码
		if err := s.redisClient.Del(context.Background(), redisKey).Err(); err != nil {
			log.Printf("删除过期验证码失败: %v", err)
		}
		return false, nil
	}

	// 增加使用次数
	_, err = s.redisClient.Incr(context.Background(), redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("更新验证码使用次数失败: %v", err)
	}

	return true, nil
}