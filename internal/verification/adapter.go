package verification

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/go-redis/redis/v8"
)

// 验证服务接口
type Service interface {
	VerifyCode(code string) (bool, error)
}

// 许可证验证服务 - 专门为许可证系统设计
type LicenseVerificationService struct {
	redisClient *redis.Client
	debugMode   bool
	debugCodes  []string
}

func NewLicenseVerificationService(redisClient *redis.Client, debugMode bool) *LicenseVerificationService {
	return &LicenseVerificationService{
		redisClient: redisClient,
		debugMode:   debugMode,
		debugCodes:  []string{"123456", "654321", "111111"}, // 调试用验证码
	}
}

func (s *LicenseVerificationService) VerifyCode(code string) (bool, error) {
	// 1. 基础格式验证
	if !s.isValidCodeFormat(code) {
		return false, fmt.Errorf("验证码格式无效")
	}

	// 2. 调试模式处理
	if s.debugMode {
		for _, debugCode := range s.debugCodes {
			if code == debugCode {
				return true, nil
			}
		}
	}

	// 3. 验证许可证验证码
	return s.verifyLicenseCode(code)
}

func (s *LicenseVerificationService) verifyLicenseCode(code string) (bool, error) {
	// 许可证验证码的Redis Key格式：license:code:{验证码}
	redisKey := fmt.Sprintf("license:code:%s", code)

	// 检查验证码是否存在
	exists, err := s.redisClient.Exists(context.Background(), redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("检查验证码失败: %v", err)
	}

	if exists == 0 {
		return false, nil // 验证码不存在
	}

	// 验证码存在且有效（5分钟有效期，不限使用次数）
	return true, nil
}

func (s *LicenseVerificationService) isValidCodeFormat(code string) bool {
	matched, _ := regexp.MatchString(`^\d{6}$`, code)
	return matched
}

// 生成许可证验证码（供管理员或其他系统调用）
func (s *LicenseVerificationService) GenerateLicenseCode(code string) error {
	if !s.isValidCodeFormat(code) {
		return fmt.Errorf("验证码格式无效，必须是6位数字")
	}

	// 许可证验证码的Redis Key格式：license:code:{验证码}
	redisKey := fmt.Sprintf("license:code:%s", code)

	// 存储验证码，有效期5分钟
	err := s.redisClient.Set(context.Background(), redisKey, "valid", 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("存储验证码失败: %v", err)
	}

	return nil
}
