package main

import (
	"my-finance-backend/config"
	"os"

	"bufio"
	"log"
	"strings"
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

// getEnv gets environment variable with a default value, and reads from .env file if not set
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		value = getEnvFromFile(key, defaultValue)
	}
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvFromFile reads the .env file and gets the value for the given key
func getEnvFromFile(key, defaultValue string) string {
	file, err := os.Open(".env")
	if err != nil {
		log.Printf("Error opening .env file: %v", err)
		return defaultValue
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key+"=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading .env file: %v", err)
	}

	return defaultValue
}
