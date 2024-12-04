package main

import (
	"log"
	"my-finance-backend/config"
	"os"

	"github.com/joho/godotenv"
)

// LoadConfig loads configuration from environment variables
func LoadConfig() *config.Config {
	// Load .env file if APP_ENV is not set
	if os.Getenv("APP_ENV") == "" {
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env file not found")
		}
	}

	// If APP_ENV is production, load .env.prod
	if os.Getenv("APP_ENV") == "production" {
		if err := godotenv.Load(".env.prod"); err != nil {
			log.Fatal("Error loading .env.prod file")
		}
	}

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
