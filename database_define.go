package main

import (
	"my-finance-backend/config"
	"os"
)

// LoadConfig loads configuration from environment variables
func LoadConfig() *config.Config {

	config := &config.Config{
		AppEnv:                   getEnv("APP_ENV", "development"),
		DatabaseURL:              getEnv("DATABASE_URL", "mongodb://localhost:27017"),
		DatabaseName:             getEnv("DATABASE_NAME", "MyFinance_Dev"),
		JWTSecret:                getEnv("JWT_SECRET", "your-dev-secret-key"),
		CollectionUserName:       "users",
		CollectionExpensesName:   "expenses",
		CollectionCategoriesName: "categories",
		CollectionTagsName:       "tags",
	}

	return config
}

// getEnv gets environment variable with a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
