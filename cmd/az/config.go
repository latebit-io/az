package main

import (
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	AllowedOrigins     []string
	BootstrapApiKey    string
	CORSEnabled        bool
	DbConnection       string
	DecisionLogEnabled bool
	DefaultTenantID    string
	Domain             string
	Port               int
	RequestsPerSecond  int
}

func NewAppConfig() (*AppConfig, error) {
	config := &AppConfig{}

	config.Port = getEnvAsInt("PORT", 8080)
	config.DbConnection = getEnv("DB_CONNECTION", "postgres://az:az@localhost:5432/az?sslmode=disable")
	config.Domain = getEnv("DOMAIN", "")
	config.AllowedOrigins = getEnvAsStringSlice("ALLOWED_WEB_ORIGINS", []string{})
	config.BootstrapApiKey = getEnv("BOOTSTRAP_API_KEY", "")
	config.CORSEnabled = getEnv("CORS_ENABLED", "false") == "true"
	config.DecisionLogEnabled = getEnv("DECISION_LOG_ENABLED", "true") == "true"
	config.RequestsPerSecond = getEnvAsInt("REQUESTS_PER_SECOND", 20)
	config.DefaultTenantID = getEnv("DEFAULT_TENANT_ID", "default")

	return config, nil
}

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, ",")
}
