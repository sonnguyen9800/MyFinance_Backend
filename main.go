package main

import (
	"context"
	"log"
	"my-finance-backend/authentication"
	"my-finance-backend/category"
	"my-finance-backend/expense"
	"my-finance-backend/tag"
	"my-finance-backend/users"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"errors"
)

// Authentication middleware
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			config := LoadConfig()
			return []byte(config.JWTSecret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("user_id", claims["user_id"])
			c.Set("email", claims["email"])
			c.Set("role", claims["role"])
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}
	}
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Initialize MongoDB connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.DatabaseURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connected to MongoDB! Environment: %s, Database: %s\n", config.AppEnv, config.DatabaseName)

	// Initialize handlers
	authHandler := authentication.NewHandler(client, config, []byte(config.JWTSecret))
	expenseHandler := expense.NewHandler(client, config, []byte(config.JWTSecret))

	categoryHandler := category.NewHandler(client, config, []byte(config.JWTSecret))
	tagHandler := tag.NewHandler(client, config)
	// Initialize Gin router
	r := gin.Default()

	// Public routes
	r.POST("/api/login", authHandler.HandleLogin)
	r.POST("/api/signin", authHandler.HandleLogin)
	r.POST("/api/signup", authHandler.HandleSignup)

	// Login by token
	userAuthen := users.NewHandler(client, []byte(config.JWTSecret))
	r.POST("/api/user", userAuthen.HandleLoginByToken)

	// Protected routes
	auth := r.Group("/api")
	auth.Use(authMiddleware())
	{
		// Category routes
		auth.POST("/categories", categoryHandler.HandleCreateCategory)
		auth.GET("/categories", categoryHandler.HandleGetCategories)
		auth.GET("/categories/:id", categoryHandler.HandleGetCategory)
		auth.PUT("/categories/:id", categoryHandler.HandleUpdateCategory)
		auth.DELETE("/categories/:id", categoryHandler.HandleDeleteCategory)

		// Tag routes
		r.POST("/api/tags", tagHandler.HandleCreateTag)
		r.GET("/api/tags", tagHandler.HandleGetTags)
		r.GET("/api/tags/:id", tagHandler.HandleGetTag)

		// Expense routes
		auth.POST("/expenses", expenseHandler.HandleCreateExpense)
		auth.GET("/expenses", expenseHandler.HandleGetExpenses)
		auth.GET("/expenses_last", expenseHandler.HandleGetLastExpenses)
		auth.GET("/expenses_montly", expenseHandler.HandleGetExpensesMonthly)
		auth.POST("/expenses/upload", expenseHandler.HandleUploadCSV)

		auth.GET("/expenses/:id", expenseHandler.HandleGetExpense)
		auth.PUT("/expenses/:id", expenseHandler.HandleUpdateExpense)
		auth.DELETE("/expenses/:id", expenseHandler.HandleDeleteExpense)

	}

	// Start server
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}

}
