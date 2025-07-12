package license

import (
	"crypto/rand"
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
	"meea-icey/internal/verification"
)

type AdminHandler struct {
	verificationService *verification.LicenseVerificationService
}

func NewAdminHandler(verificationService *verification.LicenseVerificationService) *AdminHandler {
	return &AdminHandler{
		verificationService: verificationService,
	}
}

// 生成许可证验证码请求
type GenerateCodeRequest struct {
	Code string `json:"code,omitempty"` // 可选，如果不提供则自动生成
}

// 生成许可证验证码响应
type GenerateCodeResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// 生成许可证验证码
func (h *AdminHandler) GenerateLicenseCode(c *gin.Context) {
	var req GenerateCodeRequest

	// 绑定请求数据
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GenerateCodeResponse{
			Success: false,
			Error:   "请求格式错误",
		})
		return
	}

	var code string
	if req.Code != "" {
		// 使用指定的验证码
		code = req.Code
	} else {
		// 自动生成6位数字验证码
		generatedCode, err := generateSixDigitCode()
		if err != nil {
			c.JSON(http.StatusInternalServerError, GenerateCodeResponse{
				Success: false,
				Error:   "生成验证码失败",
			})
			return
		}
		code = generatedCode
	}

	// 生成许可证验证码
	err := h.verificationService.GenerateLicenseCode(code)
	if err != nil {
		c.JSON(http.StatusBadRequest, GenerateCodeResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, GenerateCodeResponse{
		Success: true,
		Code:    code,
		Message: "许可证验证码生成成功，有效期5分钟",
	})
}

// 生成六位随机数字验证码
func generateSixDigitCode() (string, error) {
	max := big.NewInt(900000)
	min := big.NewInt(100000)
	// 生成 [0, 900000) 之间的随机数
	randNum, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	// 加上 min，得到 [100000, 1000000) 之间的随机数
	result := new(big.Int).Add(randNum, min)
	return result.String(), nil
}
