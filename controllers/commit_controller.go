package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"meea-icey/models"
	"meea-icey/services"
)

// CommitController 处理提交相关请求
type CommitController struct {
	config        *models.Config
	verifyService *services.VerifyService
	gitService    *services.GitService
	commitService *services.CommitService
}

// NewCommitController 创建CommitController实例
func NewCommitController(config *models.Config, verifyService *services.VerifyService, gitService *services.GitService, commitService *services.CommitService) *CommitController {
	return &CommitController{
		config:        config,
		verifyService: verifyService,
		gitService:    gitService,
		commitService: commitService,
	}
}

// CommitRequest 提交信息请求参数
type CommitRequest struct {
	Subject string `json:"subject" binding:"required"`
	Content string `json:"content" binding:"required"`
	Code    string `json:"code" binding:"required"`
}

// HandleCommit 处理提交信息请求
func (c *CommitController) HandleCommit(ctx *gin.Context) {
	var req CommitRequest
	if err := ctx.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    "无效的请求参数: " + err.Error(),
			"data":   nil,
		})
		return
	}

	// 验证subject长度
	if len(req.Subject) < 6 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    "subject必须至少包含6个字符",
			"data":   nil,
		})
		return
	}

	// 调用服务层处理提交逻辑
	result, err := c.commitService.ProcessCommit(req.Subject, req.Content, req.Code)
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
		"data":   gin.H{
			"token": result,
		},
	})
}
