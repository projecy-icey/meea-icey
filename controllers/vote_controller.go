package controllers

import (
	"net/http"
	"meea-icey/services"
	"github.com/gin-gonic/gin"
)

type VoteController struct {
	voteService *services.VoteService
}

func NewVoteController(voteService *services.VoteService) *VoteController {
	return &VoteController{voteService: voteService}
}

type VoteRequest struct {
	Subject string `json:"subject" binding:"required"`
	ID      string `json:"id" binding:"required"`
	Vote    *int   `json:"vote" binding:"required"` // 1=可信，0=不可信
	Code    string `json:"code" binding:"required"`
}

type VoteResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Data    struct {
		Percent int `json:"percent"`
	} `json:"data"`
}

func (c *VoteController) HandleVote(ctx *gin.Context) {
	var req VoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    "无效的请求参数: " + err.Error(),
			"data":   nil,
		})
		return
	}
	if req.Vote == nil || (*req.Vote != 0 && *req.Vote != 1) {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    "vote 字段必须为 0 或 1",
			"data":   nil,
		})
		return
	}
	percent, err := c.voteService.Vote(req.Subject, req.ID, uint8(*req.Vote), req.Code)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":    err.Error(),
			"data":   nil,
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"msg":    "",
		"data": gin.H{
			"percent": percent,
		},
	})
} 