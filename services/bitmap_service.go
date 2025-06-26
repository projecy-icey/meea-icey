package services

import (
	"fmt"
	"math"
	"os"
	"sync"
)

type BitmapService struct {
	gitService *GitService
	mu         sync.Mutex
}

const (
	blockSize = 256
	threshold = 0.8
)

func NewBitmapService(gitService *GitService) *BitmapService {
	return &BitmapService{
		gitService: gitService,
	}
}

// AddBit 添加bit到bitmap
func (s *BitmapService) AddBit(bmFile, bmiFile string, value uint8) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 读取bm和bmi文件
	bm, err := s.readBitmap(bmFile)
	if err != nil {
		return err
	}

	bmi, err := s.readBitmap(bmiFile)
	if err != nil {
		return err
	}

	// 2. 从第256位开始查找可用位置
	startPos := blockSize
	pos := startPos
	for ; pos < len(bm); pos++ {
		if bmi[pos] == 0 {
			break
		}
	}

	// 3. 如果256-511位置已满，处理后续块
	if pos >= 2*blockSize {
		// 统计后续256位的使用情况
		count := 0
		for i := 2 * blockSize; i < 3*blockSize; i++ {
			if bmi[i] == 1 && bm[i] == 1 {
				count++
			}
		}

		// 根据统计结果决定如何处理
		if count >= int(math.Floor(float64(blockSize)*threshold)) {
			// 超过80%，将后续256位的0标记为1
			for i := 2 * blockSize; i < 3*blockSize; i++ {
				if bm[i] == 0 {
					bm[i] = 1
					bmi[i] = 1
				}
			}
		} else {
			// 否则清空后续256位
			for i := 2 * blockSize; i < 3*blockSize; i++ {
				bm[i] = 0
				bmi[i] = 0
			}
		}
		pos = 2 * blockSize
	}

	// 4. 如果前256位已满，查找相反位置替换
	if pos >= len(bm) {
		target := uint8(0)
		if value == 0 {
			target = 1
		}

		for i := 0; i < blockSize; i++ {
			if bm[i] == target && bmi[i] == 1 {
				bm[i] = value
				pos = i
				break
			}
		}
	}

	// 5. 写入bit
	bm[pos] = value
	bmi[pos] = 1

	// 6. 保存文件
	if err := s.writeBitmap(bmFile, bm); err != nil {
		return err
	}
	return s.writeBitmap(bmiFile, bmi)
}

// GetStats 统计bitmap结果
func (s *BitmapService) GetStats(bmFile, bmiFile string) (int, error) {
	bm, err := s.readBitmap(bmFile)
	if err != nil {
		return 0, err
	}

	bmi, err := s.readBitmap(bmiFile)
	if err != nil {
		return 0, err
	}

	// 检查前256位是否有值
	hasValue := false
	count := 0
	total := 0

	for i := 0; i < blockSize; i++ {
		if bmi[i] == 1 {
			hasValue = true
			if bm[i] == 1 {
				count++
			}
			total++
		}
	}

	// 如果前256位有值，统计前256位
	if hasValue && total > 0 {
		return int(float64(count) / float64(total) * 100), nil
	}

	// 否则统计后续256位
	count = 0
	total = 0
	for i := blockSize; i < 2*blockSize; i++ {
		if bmi[i] == 1 {
			if bm[i] == 1 {
				count++
			}
			total++
		}
	}

	if total == 0 {
		return 0, nil
	}
	return int(float64(count) / float64(total) * 100), nil
}

// 读取bitmap文件
func (s *BitmapService) readBitmap(filePath string) ([]uint8, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在时创建空bitmap
			return make([]uint8, 1024), nil
		}
		return nil, err
	}
	return data, nil
}

// 写入bitmap文件
func (s *BitmapService) writeBitmap(filePath string, data []uint8) error {
	return os.WriteFile(filePath, data, 0644)
}

// LockBitmaps 锁定bitmap文件
func (s *BitmapService) LockBitmaps(bmFile, bmiFile string) error {
	// 先尝试解锁，忽略错误，防止残留锁
	_ = s.gitService.UnlockFile(bmFile)
	_ = s.gitService.UnlockFile(bmiFile)

	// 检查文件是否存在
	if _, err := os.Stat(bmFile); os.IsNotExist(err) {
		return fmt.Errorf("bm文件不存在: %s", bmFile)
	}
	if _, err := os.Stat(bmiFile); os.IsNotExist(err) {
		return fmt.Errorf("bmi文件不存在: %s", bmiFile)
	}

	// 锁定bm文件
	if err := s.gitService.LockFile(bmFile); err != nil {
		return fmt.Errorf("锁定bm文件失败: %v", err)
	}

	// 锁定bmi文件
	if err := s.gitService.LockFile(bmiFile); err != nil {
		// 回滚: 解锁已锁定的bm文件
		if unlockErr := s.gitService.UnlockFile(bmFile); unlockErr != nil {
			return fmt.Errorf("锁定bmi文件失败: %v, 且回滚解锁bm文件也失败: %v", err, unlockErr)
		}
		return fmt.Errorf("锁定bmi文件失败: %v", err)
	}

	return nil
}

// UnlockBitmaps 解锁bitmap文件
func (s *BitmapService) UnlockBitmaps(bmFile, bmiFile string) error {
	var errs []error

	// 解锁bm文件
	if err := s.gitService.UnlockFile(bmFile); err != nil {
		errs = append(errs, fmt.Errorf("解锁bm文件失败: %v", err))
		fmt.Printf("[UnlockBitmaps] 解锁bm文件失败: %s, err: %v\n", bmFile, err)
	} else {
		fmt.Printf("[UnlockBitmaps] 已解锁bm文件: %s\n", bmFile)
	}

	// 解锁bmi文件
	if err := s.gitService.UnlockFile(bmiFile); err != nil {
		errs = append(errs, fmt.Errorf("解锁bmi文件失败: %v", err))
		fmt.Printf("[UnlockBitmaps] 解锁bmi文件失败: %s, err: %v\n", bmiFile, err)
	} else {
		fmt.Printf("[UnlockBitmaps] 已解锁bmi文件: %s\n", bmiFile)
	}

	if len(errs) > 0 {
		return fmt.Errorf("解锁bitmap文件时发生错误: %v", errs)
	}
	return nil
}

// IsLocked 检查文件是否被锁定
func (s *BitmapService) IsLocked(filePath string) (bool, error) {
	return s.gitService.IsFileLocked(filePath)
}

// CommitBitmap 提交bitmap变更
func (s *BitmapService) CommitBitmap(filePath, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查文件是否被锁定
	locked, err := s.IsLocked(filePath)
	if err != nil {
		return fmt.Errorf("检查文件锁定状态失败: %v", err)
	}
	if !locked {
		return fmt.Errorf("文件未被锁定: %s", filePath)
	}

	// 提交变更
	if err := s.gitService.CommitFile(filePath, message); err != nil {
		return fmt.Errorf("提交文件变更失败: %v", err)
	}

	// 解锁文件
	if err := s.gitService.UnlockFile(filePath); err != nil {
		return fmt.Errorf("解锁文件失败: %v", err)
	}

	return nil
}