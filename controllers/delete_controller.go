package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"meea-icey/services"
)

// DeleteController 处理删除相关请求
type DeleteController struct {
	deleteService *services.DeleteService
}

// NewDeleteController 创建DeleteController实例
func NewDeleteController(deleteService *services.DeleteService) *DeleteController {
	return &DeleteController{
		deleteService: deleteService,
	}
}

// DeleteReq 删除信息请求参数
type DeleteReq struct {
	Subject string `json:"subject" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Token   string `json:"token" binding:"required"`
}

// HandleDelete 处理删除请求
func (c *DeleteController) HandleDelete(ctx *gin.Context) {
	var req DeleteReq
	if err := ctx.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    "无效的请求参数: " + err.Error(),
			"data":   nil,
		})
		return
	}

	err := c.deleteService.ProcessDelete(req.Subject, req.Code, req.Token)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"msg":    err.Error(),
			"data":   nil,
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"msg":    "",
		"data":   nil,
	})
}