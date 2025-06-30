package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// GitService 封装Git相关操作
type GitService struct {
	clonePath     string
	repositoryURL string
	sshKey        string
}

// NewGitService 创建GitService实例
// 参数:
//
//	clonePath: 仓库克隆目标路径
//	repositoryURL: 远程仓库URL
//	sshKey: SSH私钥内容
//
// 返回:
//
//	初始化成功的GitService实例和nil错误；若SSH密钥为空则返回nil和错误信息
func NewGitService(clonePath, repositoryURL, sshKey string) (*GitService, error) {
	if sshKey == "" {
		return nil, fmt.Errorf("SSH密钥不能为空")
	}
	return &GitService{
		clonePath:     clonePath,
		repositoryURL: repositoryURL,
		sshKey:        sshKey,
	}, nil
}

// getSSHAuth 获取SSH认证配置
func (g *GitService) getSSHAuth() (*gitssh.PublicKeys, error) {
	if g.sshKey == "" {
		return nil, errors.New("SSH密钥未配置")
	}

	keyContent := g.sshKey
	// 如果 sshKey 是文件路径，读取内容
	if _, err := os.Stat(g.sshKey); err == nil {
		data, err := os.ReadFile(g.sshKey)
		if err != nil {
			return nil, fmt.Errorf("读取SSH密钥文件失败: %v", err)
		}
		keyContent = string(data)
	}

	// 解析私钥
	key, err := cryptossh.ParsePrivateKey([]byte(keyContent))
	if err != nil {
		return nil, fmt.Errorf("解析SSH私钥失败: %v", err)
	}

	// 优先级：环境变量 > /app/.ssh/known_hosts > ~/.ssh/known_hosts
	knownHostsPath := os.Getenv("SSH_KNOWN_HOSTS")
	if knownHostsPath == "" {
		if _, err := os.Stat("/app/.ssh/known_hosts"); err == nil {
			knownHostsPath = "/app/.ssh/known_hosts"
		} else {
			home, err := os.UserHomeDir()
			if err == nil {
				knownHostsPath = home + "/.ssh/known_hosts"
			}
		}
	}
	callback, err := gitssh.NewKnownHostsCallback(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("加载 known_hosts 失败: %v", err)
	}

	auth := &gitssh.PublicKeys{
		User:   "git",
		Signer: key,
	}
	auth.HostKeyCallback = callback

	return auth, nil
}

// BuildSubjectPath 构建基于subject的目录路径
func (g *GitService) BuildSubjectPath(subject string) (string, string) {
	if len(subject) < 6 {
		return "", ""
	}

	part1 := subject[:2]
	part2 := subject[2:4]
	part3 := subject[4:6]

	// 完整路径
	fullPath := filepath.Join(g.clonePath, "icey-storage", part1, part2, part3, subject)
	// 相对路径
	relativePath := filepath.Join(part1, part2, part3, subject)

	return fullPath, relativePath
}

// CloneRepository 使用SSH认证克隆Git仓库
// 参数:
//
//	dirName: 目标目录路径
//
// 返回:
//
//	克隆成功返回nil；若SSH密钥未配置或克隆失败则返回相应错误
func (g *GitService) CloneRepository() error {
	if g.sshKey == "" {
		return errors.New("SSH密钥未配置")
	}

	// 统一仓库目录为 clonePath/icey-storage
	fullPath := filepath.Join(g.clonePath, "icey-storage")

	// 检查目录是否已存在
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		// 目录已存在，直接返回成功，不报错
		return nil
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("创建父目录失败: %v", err)
	}

	// 获取SSH认证
	auth, err := g.getSSHAuth()
	if err != nil {
		return err
	}

	// 克隆仓库到指定目录
	_, err = git.PlainClone(fullPath, false, &git.CloneOptions{
		URL:  g.repositoryURL,
		Auth: auth,
	})
	if err != nil {
		return fmt.Errorf("克隆仓库失败: %v", err)
	}

	return nil
}

// PullRepository 拉取Git仓库最新代码
// 参数:
//
//	dirName: 本地仓库目录名称
//
// 返回:
//
//	拉取成功返回nil；若目录不存在、仓库验证失败或拉取失败则返回相应错误
func (g *GitService) PullRepository(dirName string) error {
	if g.sshKey == "" {
		return fmt.Errorf("SSH密钥未配置")
	}
	// 统一仓库目录为 clonePath/icey-storage
	fullPath := filepath.Join(g.clonePath, "icey-storage")

	// 检查目录是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// 目录不存在，执行克隆
		if err := g.CloneRepository(); err != nil {
			return fmt.Errorf("clone repository failed: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("check directory failed: %v", err)
	}

	// 目录存在，检查是否为Git仓库
	gitDir := filepath.Join(fullPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("directory exists but is not a Git repository: %s", fullPath)
	} else if err != nil {
		return fmt.Errorf("check Git repository failed: %v", err)
	}

	// 打开现有仓库
	r, err := git.PlainOpen(fullPath)
	if err != nil {
		return fmt.Errorf("打开仓库失败: %v", err)
	}

	// 获取工作目录
	worktree, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}

	// 获取SSH认证
	auth, err := g.getSSHAuth()
	if err != nil {
		return err
	}

	// 拉取最新代码
	err = worktree.PullContext(context.Background(), &git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("拉取代码失败: %v", err)
	}

	return nil
}

// getRepoRoot 获取仓库根目录
func (g *GitService) getRepoRoot() string {
	return filepath.Join(g.clonePath, "icey-storage")
}

// LockFile 锁定Git LFS文件
func (g *GitService) LockFile(filePath string) error {
	repoRoot := g.getRepoRoot()
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(repoRoot, filePath)
		if err != nil {
			fmt.Printf("[LockFile] 路径转换失败: repoRoot=%s, filePath=%s, err=%v\n", repoRoot, filePath, err)
			return fmt.Errorf("转换为相对路径失败: %v", err)
		}
	}
	fmt.Printf("[LockFile] repoRoot=%s\n", repoRoot)
	fmt.Printf("[LockFile] filePath=%s\n", filePath)
	fmt.Printf("[LockFile] relPath=%s\n", relPath)
	cmd := exec.Command("git", "lfs", "lock", relPath)
	cmd.Dir = repoRoot
	sshKeyPath := g.sshKey
	if sshKeyPath != "" && !filepath.IsAbs(sshKeyPath) {
		sshKeyPath = filepath.Join(repoRoot, sshKeyPath)
	}
	cmd.Env = append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
	)
	fmt.Printf("[LockFile] 执行命令: cd %s && GIT_SSH_COMMAND=ssh -i '%s' -o IdentitiesOnly=yes git lfs lock %s\n", repoRoot, sshKeyPath, relPath)
	output, err := cmd.CombinedOutput()
	fmt.Printf("[LockFile] 命令输出: %s\n", string(output))
	if err != nil && string(output) != "" && strings.Contains(string(output), "Lock exists") {
		// 检查锁 owner
		owner := ""
		lockCmd := exec.Command("git", "lfs", "locks")
		lockCmd.Dir = repoRoot
		lockCmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
		)
		locksOut, _ := lockCmd.CombinedOutput()
		lines := strings.Split(string(locksOut), "\n")
		for _, line := range lines {
			if strings.Contains(line, relPath) {
				// 解析 owner
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					owner = fields[2]
				}
				break
			}
		}
		// 获取当前用户名
		userCmd := exec.Command("git", "config", "user.name")
		userCmd.Dir = repoRoot
		userCmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
		)
		userOut, _ := userCmd.CombinedOutput()
		currentUser := strings.TrimSpace(string(userOut))
		if owner != "" && currentUser != "" && strings.Contains(owner, currentUser) {
			// 是自己锁的，自动解锁再重试
			unlockCmd := exec.Command("git", "lfs", "unlock", relPath)
			unlockCmd.Dir = repoRoot
			unlockCmd.Env = append(os.Environ(),
				"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
			)
			unlockOut, unlockErr := unlockCmd.CombinedOutput()
			fmt.Printf("[LockFile] unlock输出: %s\n", string(unlockOut))
			if unlockErr != nil {
				return fmt.Errorf("自动解锁失败: %v, 输出: %s", unlockErr, string(unlockOut))
			}
			// 再次尝试加锁
			cmd2 := exec.Command("git", "lfs", "lock", relPath)
			cmd2.Dir = repoRoot
			cmd2.Env = append(os.Environ(),
				"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
			)
			output2, err2 := cmd2.CombinedOutput()
			fmt.Printf("[LockFile] retry lock输出: %s\n", string(output2))
			if err2 != nil {
				return fmt.Errorf("重试加锁失败: %v, 输出: %s", err2, string(output2))
			}
			return nil
		}
		return fmt.Errorf("锁定文件失败: 已被他人占用，请稍后重试")
	}
	if err != nil {
		fmt.Printf("[LockFile] 错误: %v\n", err)
		return fmt.Errorf("锁定文件失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// UnlockFile 解锁Git LFS文件
func (g *GitService) UnlockFile(filePath string) error {
	repoRoot := g.getRepoRoot()
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(repoRoot, filePath)
		if err != nil {
			return fmt.Errorf("转换为相对路径失败: %v", err)
		}
	}
	cmd := exec.Command("git", "lfs", "unlock", relPath)
	cmd.Dir = repoRoot
	sshKeyPath := g.sshKey
	if sshKeyPath != "" && !filepath.IsAbs(sshKeyPath) {
		sshKeyPath = filepath.Join(repoRoot, sshKeyPath)
	}
	cmd.Env = append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
	)
	output, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(output), "Cannot unlock file with uncommitted changes") {
		// 自动加 --force 再解锁
		forceCmd := exec.Command("git", "lfs", "unlock", "--force", relPath)
		forceCmd.Dir = repoRoot
		forceCmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
		)
		forceOut, forceErr := forceCmd.CombinedOutput()
		if forceErr != nil {
			return fmt.Errorf("强制解锁文件失败: %v, 输出: %s", forceErr, string(forceOut))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("解锁文件失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// CommitFile 提交单个文件变更
func (g *GitService) CommitFile(filePath, message string) error {
	// 获取仓库目录和相对路径
	repoDir := filepath.Dir(filePath)
	relativePath := filepath.Base(filePath)
	return g.CommitChanges(repoDir, []string{relativePath}, message)
}

// CommitChanges stages, commits and pushes changes to the Git repository
func (g *GitService) CommitChanges(repoDir string, files []string, commitMsg string) error {
	if g.sshKey == "" {
		return errors.New("SSH密钥未配置")
	}

	fullRepoPath := filepath.Join(g.clonePath, repoDir)

	// 打开仓库
	r, err := git.PlainOpen(fullRepoPath)
	if err != nil {
		return fmt.Errorf("打开仓库失败: %v", err)
	}

	// 获取工作区
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("获取工作区失败: %v", err)
	}

	// 处理文件变更（添加新文件或删除已删除的文件）
	for _, file := range files {
		fullPath := filepath.Join(fullRepoPath, file)

		// 检查文件是否存在
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// 文件不存在，说明是删除操作，使用git rm
			if _, err := w.Remove(file); err != nil {
				return fmt.Errorf("从Git中移除文件失败: %v", err)
			}
		} else {
			// 文件存在，添加到暂存区
			if _, err := w.Add(file); err != nil {
				return fmt.Errorf("添加文件到暂存区失败: %v", err)
			}
		}
	}

	// 提交变更
	_, err = w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "meea-icey",
			Email: "meea-icey@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("提交变更失败: %v", err)
	}

	// 获取SSH认证
	auth, err := g.getSSHAuth()
	if err != nil {
		return err
	}

	// 推送变更
	err = r.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("推送变更失败: %v", err)
	}

	return nil
}

// IsFileLocked 检查文件是否被锁定
func (g *GitService) IsFileLocked(filePath string) (bool, error) {
	repoRoot := g.getRepoRoot()
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(repoRoot, filePath)
		if err != nil {
			return false, fmt.Errorf("转换为相对路径失败: %v", err)
		}
	}
	cmd := exec.Command("git", "lfs", "locks")
	cmd.Dir = repoRoot
	sshKeyPath := g.sshKey
	if sshKeyPath != "" && !filepath.IsAbs(sshKeyPath) {
		sshKeyPath = filepath.Join(repoRoot, sshKeyPath)
	}
	cmd.Env = append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -i '"+sshKeyPath+"' -o IdentitiesOnly=yes",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("检查文件锁定状态失败: %v, 输出: %s", err, string(output))
	}
	return strings.Contains(string(output), relPath), nil
}

// GetClonePath 获取克隆路径
func (g *GitService) GetClonePath() string {
	return g.clonePath
}

// CheckSHA256Directory checks if the directory structure derived from SHA256 hash exists
func (g *GitService) CheckSHA256Directory(sha256 string) (bool, error) {
	if len(sha256) < 6 {
		return false, errors.New("sha256 hash must be at least 6 characters long")
	}

	// Split first 6 characters into three parts
	part1 := sha256[:2]
	part2 := sha256[2:4]
	part3 := sha256[4:6]

	// Construct the directory path with icey-storage
	dirPath := filepath.Join(g.clonePath, "icey-storage", part1, part2, part3, sha256)

	// Check if the directory exists
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check directory existence: %v", err)
	}

	return info.IsDir(), nil
}
