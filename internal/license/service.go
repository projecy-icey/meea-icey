package license

import (
	"encoding/json"
	"regexp"
	"time"

	"meea-icey/internal/crypto"
	"meea-icey/internal/verification"
)

type Service struct {
	cryptoService       *crypto.Service
	verificationService verification.Service
}

func NewService(cryptoService *crypto.Service, verificationService verification.Service) *Service {
	return &Service{
		cryptoService:       cryptoService,
		verificationService: verificationService,
	}
}

func (s *Service) ProcessLicenseRequest(encryptedData string) *LicenseResponse {
	// 1. 解密客户端数据
	decryptedString, err := s.cryptoService.DecryptClientData(encryptedData)
	if err != nil {
		return &LicenseResponse{
			Success: false,
			Error:   "数据解密失败",
			Code:    ErrCodeDecryptionFailed,
		}
	}

	// 2. 解析请求数据
	var requestData LicenseRequest
	if err := json.Unmarshal([]byte(decryptedString), &requestData); err != nil {
		return &LicenseResponse{
			Success: false,
			Error:   "请求数据格式无效",
			Code:    ErrCodeInvalidFormat,
		}
	}

	// 3. 验证请求数据格式
	if !s.validateRequestData(&requestData) {
		return &LicenseResponse{
			Success: false,
			Error:   "请求数据格式无效",
			Code:    ErrCodeInvalidFormat,
		}
	}

	// 4. 验证验证码
	isValid, err := s.verificationService.VerifyCode(requestData.VerificationCode)
	if err != nil || !isValid {
		return &LicenseResponse{
			Success: false,
			Error:   "验证码无效",
			Code:    ErrCodeInvalidCode,
		}
	}

	// 5. 生成证书
	certificate, err := s.generateCertificate(&requestData)
	if err != nil {
		return &LicenseResponse{
			Success: false,
			Error:   "证书生成失败",
			Code:    ErrCodeCertGenFailed,
		}
	}

	return &LicenseResponse{
		Success:     true,
		Certificate: certificate,
		Message:     "许可证生成成功",
	}
}

func (s *Service) validateRequestData(data *LicenseRequest) bool {
	// 验证验证码格式 (6位数字)
	codePattern := regexp.MustCompile(`^\d{6}$`)
	if !codePattern.MatchString(data.VerificationCode) {
		return false
	}

	// 验证机器码格式 (32位十六进制)
	machinePattern := regexp.MustCompile(`^[A-F0-9]{32}$`)
	if !machinePattern.MatchString(data.MachineID) {
		return false
	}

	return true
}

func (s *Service) generateCertificate(requestData *LicenseRequest) (*Certificate, error) {
	// 创建证书数据
	certData := CertificateData{
		MachineID:        requestData.MachineID,
		VerificationCode: requestData.VerificationCode,
		IssuedAt:         time.Now().UnixMilli(),
		Version:          "1.0.0",
		Issuer:           "MEEA-VIOFO-LICENSE-SERVER",
	}

	// 生成数字签名
	signature, err := s.cryptoService.SignCertificate(certData)
	if err != nil {
		return nil, err
	}

	return &Certificate{
		Data:      certData,
		Signature: signature,
	}, nil
}
