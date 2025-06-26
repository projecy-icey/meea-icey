package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"meea-icey/models"
)

type CommitService struct {
	config        *models.Config
	verifyService *VerifyService
	gitService    *GitService
}

func NewCommitService(config *models.Config, verifyService *VerifyService, gitService *GitService) *CommitService {
	return &CommitService{
		config:        config,
		verifyService: verifyService,
		gitService:    gitService,
	}
}

func (c *CommitService) ProcessCommit(subject, content, code string) (string, error) {
	// 验证subject长度
	if len(subject) < 6 {
		return "", fmt.Errorf("subject必须至少包含6个字符")
	}

	// 验证验证码
	valid, err := c.verifyService.VerifyCode(subject, code)
	if err != nil {
		return "", fmt.Errorf("验证码验证失败: %v", err)
	}
	if !valid {
		return "", fmt.Errorf("验证码无效或已过期")
	}

	// 验证码验证通过，先拉取最新代码
	if err := c.gitService.PullRepository("icey-storage"); err != nil {
		return "", fmt.Errorf("拉取仓库失败: %v", err)
	}

	// 生成文件名前缀
	fileNamePrefix, err := GenerateFileNamePrefix()
	if err != nil {
		return "", fmt.Errorf("生成文件名前缀失败: %v", err)
	}

	// 构建目录路径
	dirPath, relativePath := BuildSubjectPath(c.config.Repository.ClonePath, subject)
	if dirPath == "" {
		return "", fmt.Errorf("无效的subject格式")
	}

	// 创建目录
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 保存content到.sj文件
	sjFilePath := filepath.Join(dirPath, fileNamePrefix+".sj")
	if err := CreateFileWithContent(sjFilePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入SJ文件失败: %v", err)
	}

	// 创建512字节的bitmap文件 (.bm)
	bmFilePath := filepath.Join(dirPath, fileNamePrefix+".bm")
	bmData := make([]byte, 512)
	if err := CreateFileWithContent(bmFilePath, bmData, 0644); err != nil {
		return "", fmt.Errorf("写入BM文件失败: %v", err)
	}

	// 创建512字节的bitmap文件 (.bmi)
	bmiFilePath := filepath.Join(dirPath, fileNamePrefix+".bmi")
	bmiData := make([]byte, 512)
	if err := CreateFileWithContent(bmiFilePath, bmiData, 0644); err != nil {
		return "", fmt.Errorf("写入BMI文件失败: %v", err)
	}

	// 生成36位随机token
	token := GenerateRandomToken(36)
	// 从文件名前缀中提取ID
	parts := strings.Split(fileNamePrefix, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("文件名前缀格式错误")
	}
	id := parts[1]
	
	// 使用-拼接token和id
	fullToken := token + "-" + id
	
	// 创建.dt文件
	dtFilePath := filepath.Join(dirPath, fileNamePrefix+".dt")
	if err := CreateTokenFile(dtFilePath, subject, token); err != nil {
		return "", fmt.Errorf("写入DT文件失败: %v", err)
	}

	// 提交到Git仓库 - 确保所有文件都被提交
	commitMsg := fmt.Sprintf("%s-%s", subject, id)
	filesToCommit := []string{
		filepath.Join(relativePath, fileNamePrefix+".sj"),
		filepath.Join(relativePath, fileNamePrefix+".bm"),
		filepath.Join(relativePath, fileNamePrefix+".bmi"),
		filepath.Join(relativePath, fileNamePrefix+".dt"),
	}

	// 调试日志：打印完整文件路径和内容
	log.SetOutput(os.Stdout)
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("===== 提交前文件状态 =====")
	for _, file := range filesToCommit {
		fullPath := filepath.Join(c.config.Repository.ClonePath, "icey-storage", file)
		logger.Printf("文件路径: %s\n存在状态: %v\n大小: %d bytes\n",
			fullPath, FileExists(fullPath), GetFileSize(fullPath))

		// 检查文件内容
		if FileExists(fullPath) {
			content, _ := os.ReadFile(fullPath)
			logger.Printf("文件内容(前64字节): %x\n", content[:Min(len(content), 64)])
		}
	}

	if err := c.gitService.CommitChanges("icey-storage", filesToCommit, commitMsg); err != nil {
		return "", fmt.Errorf("Git提交失败: %v", err)
	}

	// 提交后验证文件状态
	logger.Println("===== 提交后文件状态 =====")
	for _, file := range filesToCommit {
		fullPath := filepath.Join(c.config.Repository.ClonePath, "icey-storage", file)
		logger.Printf("文件路径: %s\n存在状态: %v\n大小: %d bytes\n",
			fullPath, FileExists(fullPath), GetFileSize(fullPath))
	}

	// 返回拼接后的完整token
	return fullToken, nil
}
