package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"meea-icey/models"
)

type DeleteService struct {
	config        *models.Config
	verifyService *VerifyService
	gitService    *GitService
}

func NewDeleteService(config *models.Config, verifyService *VerifyService, gitService *GitService) *DeleteService {
	return &DeleteService{
		config:        config,
		verifyService: verifyService,
		gitService:    gitService,
	}
}

func (d *DeleteService) ProcessDelete(subject string, code string, token string) error {
	log.SetOutput(os.Stdout)
	logger := log.New(os.Stdout, "[DELETE] ", log.LstdFlags)
	logger.Printf("开始处理删除请求: subject=%s, code=%s", subject, code)
	
	// 1. 验证验证码
	valid, err := d.verifyService.VerifyCode(subject, code)
	if err != nil {
		return fmt.Errorf("验证码验证失败: %v", err)
	}
	if !valid {
		return fmt.Errorf("验证码无效或已过期")
	}
	logger.Printf("验证码验证通过")

	// 2. 从token中分割出文件id
	tokenStr, fileId, err := ParseTokenAndID(token)
	if err != nil {
		return fmt.Errorf("解析token失败: %v", err)
	}
	logger.Printf("解析文件ID: %s", fileId)

	// 3. 构建目录路径
	dirPath, relativePath := BuildSubjectPath(d.config.Repository.ClonePath, subject)
	if dirPath == "" {
		return fmt.Errorf("无效的subject格式")
	}
	logger.Printf("目标目录路径: %s", dirPath)

	// 4. 拉取仓库（如果本地没有仓库就clone，有就pull）
	if _, err := os.Stat(d.config.Repository.ClonePath); os.IsNotExist(err) {
		logger.Printf("本地仓库不存在，开始克隆")
		if err := d.gitService.CloneRepository(); err != nil {
			return fmt.Errorf("仓库克隆失败: %v", err)
		}
		logger.Printf("仓库克隆成功")
	} else {
		logger.Printf("本地仓库存在，开始拉取最新代码")
		// 注意：这里传入的是仓库名称，不是完整路径
		if err := d.gitService.PullRepository("icey-storage"); err != nil {
			return fmt.Errorf("git pull失败: %v", err)
		}
		logger.Printf("git pull成功")
	}

	// 5. 查找.dt文件
	dtFilePath := filepath.Join(dirPath, "*-"+fileId+".dt")
	dtFiles, err := filepath.Glob(dtFilePath)
	if err != nil {
		return fmt.Errorf("查找文件失败: %v", err)
	}
	if len(dtFiles) == 0 {
		logger.Printf("未找到匹配的.dt文件: %s", dtFilePath)
		return fmt.Errorf("未找到对应的文件记录")
	}
	logger.Printf("找到.dt文件: %s", dtFiles[0])

	// 6. 读取.dt文件内容并验证token
	hashedToken, err := os.ReadFile(dtFiles[0])
	if err != nil {
		return fmt.Errorf("读取token文件失败: %v", err)
	}

	// 验证token
	if err := ValidateToken(hashedToken, subject, tokenStr); err != nil {
		return fmt.Errorf("token验证失败")
	}
	logger.Printf("token验证通过")

	// 7. 删除相关文件
	filePrefix := strings.TrimSuffix(filepath.Base(dtFiles[0]), ".dt")
	filesToDelete := []string{
		filepath.Join(dirPath, filePrefix+".sj"),
		filepath.Join(dirPath, filePrefix+".bm"),
		filepath.Join(dirPath, filePrefix+".bmi"),
		dtFiles[0],
	}

	// 记录实际删除的文件
	var deletedFiles []string
	for _, file := range filesToDelete {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			logger.Printf("文件不存在，跳过: %s", file)
			continue
		}
		logger.Printf("正在删除文件: %s", file)
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("删除文件失败: %v", err)
		}
		logger.Printf("文件删除成功: %s", file)
		deletedFiles = append(deletedFiles, file)
	}

	// 8. 提交删除到远程仓库
	if len(deletedFiles) > 0 {
		filesToCommit := []string{
			filepath.Join(relativePath, filePrefix+".sj"),
			filepath.Join(relativePath, filePrefix+".bm"),
			filepath.Join(relativePath, filePrefix+".bmi"),
			filepath.Join(relativePath, filePrefix+".dt"),
		}

		logger.Printf("准备提交删除的文件: %v", filesToCommit)
		commitMsg := fmt.Sprintf("delete %s - %s", subject, filePrefix)
		if err := d.gitService.CommitChanges("icey-storage", filesToCommit, commitMsg); err != nil {
			logger.Printf("git提交失败: %v", err)
			return fmt.Errorf("git提交失败: %v", err)
		}
		logger.Printf("git提交成功: %s", commitMsg)
	} else {
		logger.Printf("没有文件被删除")
	}

	logger.Printf("删除操作完成")
	return nil
}