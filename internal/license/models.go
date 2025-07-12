package license

// 许可证请求数据 (解密后)
type LicenseRequest struct {
	VerificationCode string `json:"verificationCode" validate:"required,len=6"`
	MachineID        string `json:"machineId" validate:"required,len=32"`
}

// 加密的许可证请求
type EncryptedLicenseRequest struct {
	EncryptedData string `json:"encryptedData" validate:"required"`
}

// 证书数据 (永久有效)
type CertificateData struct {
	MachineID        string `json:"machineId"`
	VerificationCode string `json:"verificationCode"`
	IssuedAt         int64  `json:"issuedAt"`
	Version          string `json:"version"`
	Issuer           string `json:"issuer"`
}

// 完整证书
type Certificate struct {
	Data      CertificateData `json:"data"`
	Signature string          `json:"signature"`
}

// 许可证响应
type LicenseResponse struct {
	Success     bool         `json:"success"`
	Certificate *Certificate `json:"certificate,omitempty"`
	Message     string       `json:"message,omitempty"`
	Error       string       `json:"error,omitempty"`
	Code        string       `json:"code,omitempty"`
}

// 错误码定义
const (
	ErrCodeInvalidRequest      = "INVALID_REQUEST"
	ErrCodeMissingData        = "MISSING_ENCRYPTED_DATA"
	ErrCodeDecryptionFailed   = "DECRYPTION_FAILED"
	ErrCodeInvalidFormat      = "INVALID_REQUEST_FORMAT"
	ErrCodeInvalidCode        = "INVALID_VERIFICATION_CODE"
	ErrCodeCertGenFailed      = "CERTIFICATE_GENERATION_FAILED"
	ErrCodeInternalError      = "INTERNAL_ERROR"
)
