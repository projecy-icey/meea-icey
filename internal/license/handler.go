package license

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	licenseService *Service
}

func NewHandler(licenseService *Service) *Handler {
	return &Handler{
		licenseService: licenseService,
	}
}

func (h *Handler) RequestLicense(c *gin.Context) {
	var req EncryptedLicenseRequest

	// 绑定请求数据
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LicenseResponse{
			Success: false,
			Error:   "请求格式错误",
			Code:    ErrCodeInvalidRequest,
		})
		return
	}

	// 验证必要字段
	if req.EncryptedData == "" {
		c.JSON(http.StatusBadRequest, LicenseResponse{
			Success: false,
			Error:   "缺少加密数据",
			Code:    ErrCodeMissingData,
		})
		return
	}

	// 处理许可证请求
	response := h.licenseService.ProcessLicenseRequest(req.EncryptedData)

	// 返回响应
	statusCode := http.StatusOK
	if !response.Success {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, response)
}
