package models

// Config 定义配置结构体
type Config struct {
	Wechat struct {
		AppID          string `yaml:"app_id"`
		AppSecret      string `yaml:"app_secret"`
		Token          string `yaml:"token"`
		EncodingAESKey string `yaml:"encoding_aes_key"`
	} `yaml:"wechat"`
	Repository struct {
		URL       string `yaml:"url"`
		ClonePath string `yaml:"clone_path"`
		SSHKey    string `yaml:"ssh_key"`
		Username  string `yaml:"username"`
		Password  string `yaml:"password"`
	} `yaml:"repository"`
	Redis struct {
		IP       string `yaml:"ip"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
	} `yaml:"redis"`
	Verification struct {
		MaxAttempts int `yaml:"max_attempts"`
	} `yaml:"verification"`
	Server struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
	License struct {
		CommPrivateKeyPath string `yaml:"commPrivateKeyPath"`
		SignPrivateKeyPath string `yaml:"signPrivateKeyPath"`
		DebugMode          bool   `yaml:"debugMode"`
	} `yaml:"license"`
}
