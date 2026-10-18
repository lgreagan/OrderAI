package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	DeepSeekModel   string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	AppPort string
}

func Load() *Config {

	// 加载项目根目录 .env
	_ = godotenv.Load(".env")

	return &Config{
		DeepSeekAPIKey:  os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekBaseURL: getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:   getEnv("DEEPSEEK_MODEL", "deepseek-chat"),

		// 变量名需与 .env 中的 MYSQL_* 保持一致
		DBHost:     getEnv("MYSQL_HOST", "127.0.0.1"),
		DBPort:     getEnv("MYSQL_PORT", "3306"),
		DBUser:     getEnv("MYSQL_USERNAME", "root"),
		DBPassword: getEnv("MYSQL_PASSWORD", ""),
		DBName:     getEnv("MYSQL_DATABASE", "order_ai"),
		AppPort:    getEnv("APP_PORT", "8080"),
	}
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	return value
}
