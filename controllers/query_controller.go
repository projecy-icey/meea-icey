package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"meea-icey/services"
)

var sha256Regex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type QueryController struct {
	verifyService *services.VerifyService
	gitService    *services.GitService
}

func NewQueryController(verifyService *services.VerifyService, gitService *services.GitService) *QueryController {
	return &QueryController{
		verifyService: verifyService,
		gitService:    gitService,
	}
}

type QueryRequest struct {
	Subject string `json:"subject"`
	Code    string `json:"code"`
}

type QueryResponse[T any] struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg,omitempty"`
	Data    T      `json:"data,omitempty"`
}

func (c *QueryController) HandleQuery(w http.ResponseWriter, r *http.Request) {
	// 检查Content-Type是否为application/json
	if r.Header.Get("Content-Type") != "application/json" {
		resp := QueryResponse[string]{Success: false, Msg: "Content-Type必须为application/json"}
		writeJSONResponse(w, http.StatusUnsupportedMediaType, resp)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[QueryController] 解析请求体错误: %%v, 请求: %%+v", err, r.Body)
		resp := QueryResponse[string]{Success: false}
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		
		if errors.As(err, &syntaxErr) {
			resp.Msg = fmt.Sprintf("JSON语法错误在位置 %d: %v", syntaxErr.Offset, err)
		} else if errors.As(err, &typeErr) {
			resp.Msg = fmt.Sprintf("字段 '%s' 类型错误: 期望 %s", typeErr.Field, typeErr.Type)
		} else {
			resp.Msg = "无效的请求参数: " + err.Error()
		}
		writeJSONResponse(w, http.StatusBadRequest, resp)
		return
	}

	if req.Subject == "" {
		resp := QueryResponse[string]{Success: false, Msg: "subject不能为空"}
		writeJSONResponse(w, http.StatusBadRequest, resp)
		return
	}
	if req.Code == "" {
		resp := QueryResponse[string]{Success: false, Msg: "code不能为空"}
		writeJSONResponse(w, http.StatusBadRequest, resp)
		return
	}

	// 验证subject是否为64位十六进制字符串(SHA256)
	if !sha256Regex.MatchString(req.Subject) {
		resp := QueryResponse[string]{Success: false, Msg: "subject格式不正确，必须是64位十六进制字符串"}
		writeJSONResponse(w, http.StatusBadRequest, resp)
		return
	}

	// 调用验证服务进行验证码验证
	valid, err := c.verifyService.VerifyCode(req.Subject, req.Code)
	if err != nil {
		log.Printf("[QueryController] 验证码验证过程错误: %%v, subject=%%s, code=%%s", err, req.Subject, req.Code)
		resp := QueryResponse[string]{Success: false, Msg: "验证过程失败: " + err.Error()}
		writeJSONResponse(w, http.StatusInternalServerError, resp)
		return
	}

	if !valid {
		log.Printf("[QueryController] 验证码无效: subject=%%s, code=%%s", req.Subject, req.Code)
		resp := QueryResponse[string]{Success: false, Msg: "验证码错误"}
		writeJSONResponse(w, http.StatusForbidden, resp)
		return
	}

	// 验证码验证通过，执行查询逻辑
	result, err := c.executeQuery(req.Subject)
	if err != nil {
		log.Printf("[QueryController] 查询执行失败: %%v, subject=%%s", err, req.Subject)
		resp := QueryResponse[string]{Success: false, Msg: "查询执行失败"}
		writeJSONResponse(w, http.StatusInternalServerError, resp)
		return
	}

	resp := QueryResponse[[]map[string]interface{}]{Success: true, Data: result}
	writeJSONResponse(w, http.StatusOK, resp)
}

// 执行具体的查询逻辑
func (c *QueryController) executeQuery(subject string) ([]map[string]interface{}, error) {
	log.Printf("开始处理查询请求，subject: %s", subject)
	log.Printf("GitService 克隆路径: %s", c.gitService.GetClonePath())
	// 拉取代码
	if err := c.gitService.PullRepository("icey-storage"); err != nil {
		log.Printf("[QueryController] Git拉取失败: %v, 路径=%s", err, "icey-storage")
		return nil, err
	}

	// 检查SHA256目录是否存在
	exists, err := c.gitService.CheckSHA256Directory(subject)
	if err != nil {
		log.Printf("[QueryController] 检查SHA256目录失败: %v", err)
		return nil, err
	}

	if !exists {
		log.Printf("未找到任何.sj文件，返回空数组")
		return []map[string]interface{}{}, nil
	}

	// 构建完整目录路径
	dirPath, _ := services.BuildSubjectPath(c.gitService.GetClonePath(), subject)
	if dirPath == "" {
		return nil, fmt.Errorf("无效的subject格式")
	}
	log.Printf("完整查询目录路径: %s", dirPath)

	// 读取目录下所有文件
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("读取目录失败: %v, 路径: %s", err, dirPath)
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	log.Printf("找到文件数量: %d", len(entries))
	var sjFiles []string
	for i, entry := range entries {
		log.Printf("文件 %d: 名称=%s, 是否是目录=%v", i+1, entry.Name(), entry.IsDir())
		
		if !strings.HasSuffix(entry.Name(), ".sj") {
			log.Printf("跳过非.sj文件: %s", entry.Name())
			continue
		}
		
		// 解析文件名格式: 时间戳-ID
		fileNameParts := strings.Split(strings.TrimSuffix(entry.Name(), ".sj"), "-")
		if len(fileNameParts) != 2 {
			log.Printf("文件名格式不正确: %s", entry.Name())
			continue
		}
		
		sjFiles = append(sjFiles, entry.Name())
	}

	// 按时间戳排序
	sort.Slice(sjFiles, func(i, j int) bool {
		ts1 := strings.Split(sjFiles[i], "-")[0]
		ts2 := strings.Split(sjFiles[j], "-")[0]
		return ts1 > ts2
	})

	// 解析文件内容
	var results []map[string]interface{}
	for _, filename := range sjFiles {
		// 解析文件名
		prefix := strings.TrimSuffix(filename, ".sj")
		parts := strings.Split(prefix, "-")
		if len(parts) < 2 {
			continue
		}
		ts := parts[0]
		id := prefix // 这里id为完整前缀"时间戳-雪花ID"

		// 读取文件内容
		filePath := filepath.Join(dirPath, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("读取文件失败: %v, 文件: %s", err, filePath)
			continue
		}
		log.Printf("成功读取文件: %s", filePath)
		contentStr := string(content)

		// 获取bitmap统计信息（针对每条记录自己的 .bm/.bmi 文件）
		bmFile := filepath.Join(dirPath, prefix+".bm")
		bmiFile := filepath.Join(dirPath, prefix+".bmi")
		bm, _ := os.ReadFile(bmFile)
		bmi, _ := os.ReadFile(bmiFile)
		trueCount := 0
		falseCount := 0
		total := 0
		for i := 0; i < len(bmi) && i < len(bm); i++ {
			if bmi[i] == 1 {
				total++
				if bm[i] == 1 {
					trueCount++
				} else {
					falseCount++
				}
			}
		}
		percent := 0
		if total > 0 {
			percent = int(float64(trueCount) / float64(total) * 100)
		}
		conf := map[string]interface{}{
			"total": total,
			"true": trueCount,
			"false": falseCount,
			"percent": percent,
		}

		result := map[string]interface{}{
			"content": contentStr,
			"id":     id, // 返回完整前缀
			"ts":     ts,
			"conf":   conf,
		}
		results = append(results, result)
		log.Printf("成功读取文件: %s", filePath)
	}

	if len(results) == 0 {
		return []map[string]interface{}{}, nil
	}

	log.Printf("准备返回 %d 条查询结果", len(results))
	return results, nil
}

// 辅助函数：写入JSON响应
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}