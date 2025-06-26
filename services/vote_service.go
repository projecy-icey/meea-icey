package services

import (
	"fmt"
	"path/filepath"
	"meea-icey/models"
)

type VoteService struct {
	config        *models.Config
	verifyService *VerifyService
	gitService    *GitService
	bitmapService *BitmapService
}

func NewVoteService(config *models.Config, verifyService *VerifyService, gitService *GitService, bitmapService *BitmapService) *VoteService {
	return &VoteService{
		config:        config,
		verifyService: verifyService,
		gitService:    gitService,
		bitmapService: bitmapService,
	}
}

// 投票，返回最新可信比例
func (s *VoteService) Vote(subject, id string, vote uint8, code string) (int, error) {
	fmt.Printf("[Vote] subject=%s, id=%s, vote=%d, code=%s\n", subject, id, vote, code)
	// 验证 subject 和 code
	if len(subject) != 64 {
		fmt.Printf("[Vote] subject格式不正确\n")
		return 0, fmt.Errorf("subject格式不正确，必须是64位十六进制字符串")
	}
	valid, err := s.verifyService.VerifyCode(subject, code)
	if err != nil {
		fmt.Printf("[Vote] 验证码验证失败: %v\n", err)
		return 0, fmt.Errorf("验证码验证失败: %v", err)
	}
	if !valid {
		fmt.Printf("[Vote] 验证码无效或已过期\n")
		return 0, fmt.Errorf("验证码无效或已过期")
	}

	// 验证码验证通过后，先拉取/克隆仓库
	fmt.Printf("[Vote] 拉取/克隆仓库...\n")
	if err := s.gitService.PullRepository("icey-storage"); err != nil {
		fmt.Printf("[Vote] 拉取仓库失败: %v\n", err)
		return 0, fmt.Errorf("拉取仓库失败: %v", err)
	}

	// 构建 bitmap 路径
	part1 := subject[:2]
	part2 := subject[2:4]
	part3 := subject[4:6]
	dirPath := filepath.Join(s.config.Repository.ClonePath, "icey-storage", part1, part2, part3, subject)
	filePrefix := id
	bmFile := filepath.Join(dirPath, filePrefix+".bm")
	bmiFile := filepath.Join(dirPath, filePrefix+".bmi")

	fmt.Printf("[Vote] bitmap路径: bmFile=%s, bmiFile=%s\n", bmFile, bmiFile)

	// 加锁
	fmt.Printf("[Vote] 尝试加锁...\n")
	if err := s.bitmapService.LockBitmaps(bmFile, bmiFile); err != nil {
		fmt.Printf("[Vote] 加锁失败: %v\n", err)
		return 0, fmt.Errorf("锁定bitmap文件失败: %v", err)
	}
	defer func() {
		fmt.Printf("[Vote] 尝试解锁...\n")
		s.bitmapService.UnlockBitmaps(bmFile, bmiFile)
	}()

	// 添加投票
	fmt.Printf("[Vote] 添加投票... vote=%d\n", vote)
	if err := s.bitmapService.AddBit(bmFile, bmiFile, vote); err != nil {
		fmt.Printf("[Vote] 添加投票失败: %v\n", err)
		return 0, fmt.Errorf("添加投票失败: %v", err)
	}

	// 提交变更（bm和bmi都要提交）
	commitMsg := fmt.Sprintf("vote update for %s-%s", subject, id)
	fmt.Printf("[Vote] 提交变更: %s\n", commitMsg)
	if err := s.gitService.CommitChanges("icey-storage", []string{
		filepath.Join(part1, part2, part3, subject, filePrefix+".bm"),
		filepath.Join(part1, part2, part3, subject, filePrefix+".bmi"),
	}, commitMsg); err != nil {
		fmt.Printf("[Vote] Git提交失败: %v\n", err)
		return 0, fmt.Errorf("Git提交失败: %v", err)
	}

	// 统计最新结果
	fmt.Printf("[Vote] 统计最新结果...\n")
	percent, err := s.bitmapService.GetStats(bmFile, bmiFile)
	if err != nil {
		fmt.Printf("[Vote] 统计bitmap失败: %v\n", err)
		return 0, fmt.Errorf("统计bitmap失败: %v", err)
	}
	fmt.Printf("[Vote] 完成，percent=%d\n", percent)
	return percent, nil
} 