package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v3"

	"meea-icey/controllers"
	"meea-icey/internal/crypto"
	"meea-icey/internal/license"
	"meea-icey/internal/verification"
	"meea-icey/models"
	"meea-icey/services"
)

// CORS中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	// 优先加载本地开发配置
	configPath := "config.yaml"
	if _, err := os.Stat("config.local.yaml"); err == nil {
		log.Println("检测到 config.local.yaml，优先加载本地开发配置")
		configPath = "config.local.yaml"
	}

	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 初始化Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Redis.IP, config.Redis.Port),
		Password: config.Redis.Password,
		DB:       0,
	})
	ctx := context.Background()

	// 测试Redis连接
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Redis连接失败: %v", err)
	}
	log.Println("Redis连接成功")

	// 初始化服务
	// 读取SSH密钥文件
	gitService, err := services.NewGitService(config.Repository.ClonePath, config.Repository.URL, config.Repository.SSHKey)
	if err != nil {
		log.Fatalf("初始化GitService失败: %v", err)
	}
	verifyService := services.NewVerifyService(redisClient, config)

	// 初始化控制器
	wechatController := controllers.NewWechatController(config, redisClient, ctx)
	queryController := controllers.NewQueryController(verifyService, gitService)

	// 初始化Services
	commitService := services.NewCommitService(config, verifyService, gitService)
	deleteService := services.NewDeleteService(config, verifyService, gitService)
	bitmapService := services.NewBitmapService(gitService)
	voteService := services.NewVoteService(config, verifyService, gitService, bitmapService)

	// 初始化Controllers
	commitController := controllers.NewCommitController(config, verifyService, gitService, commitService)
	deleteController := controllers.NewDeleteController(deleteService)
	voteController := controllers.NewVoteController(voteService)

	// 初始化许可证系统
	cryptoService, err := crypto.NewService(
		config.License.CommPrivateKeyPath,
		config.License.SignPrivateKeyPath,
	)
	if err != nil {
		log.Printf("警告: 许可证系统初始化失败: %v", err)
		log.Println("许可证功能将不可用")
	}

	var licenseHandler *license.Handler
	var licenseAdminHandler *license.AdminHandler
	var licenseVerificationService *verification.LicenseVerificationService

	if cryptoService != nil {
		// 初始化许可证专用验证服务
		licenseVerificationService = verification.NewLicenseVerificationService(redisClient, config.License.DebugMode)

		// 初始化许可证服务
		licenseService := license.NewService(cryptoService, licenseVerificationService)
		licenseHandler = license.NewHandler(licenseService)

		// 初始化许可证管理服务
		licenseAdminHandler = license.NewAdminHandler(licenseVerificationService)
	}

	// 设置路由
	router := gin.Default()

	// 添加CORS中间件
	router.Use(CORSMiddleware())

	// 健康检查接口
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.Any("/wechat", func(c *gin.Context) {
		wechatController.HandleMessage(c.Writer, c.Request)
	})
	router.POST("/query", func(c *gin.Context) {
		queryController.HandleQuery(c.Writer, c.Request)
	})
	router.POST("/commit", commitController.HandleCommit)
	router.POST("/delete", deleteController.HandleDelete)
	router.POST("/vote", voteController.HandleVote)

	// 许可证API路由
	if licenseHandler != nil {
		v1 := router.Group("/api/v1")
		{
			v1.POST("/license", licenseHandler.RequestLicense)
		}

		// 许可证管理API路由
		if licenseAdminHandler != nil {
			admin := router.Group("/api/admin")
			{
				admin.POST("/license/generate-code", licenseAdminHandler.GenerateLicenseCode)
			}
		}

		log.Println("许可证系统已启用")
	} else {
		log.Println("许可证系统未启用")
	}

	// 启动HTTP服务器
	listenAddr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	server := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	log.Printf("服务器启动在 %s", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// 加载配置文件（支持环境变量渲染）
func loadConfig(path string) (*models.Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 环境变量渲染
	content := os.ExpandEnv(string(data))

	var config models.Config
	err = yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}
