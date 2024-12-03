package main

import (
	"context"
	"log"
	"my-finance-backend/authentication"
	"my-finance-backend/expense"
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

var (
	jwtSecret = []byte("your-secret-key") // In production, use environment variable
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
			return jwtSecret, nil
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
	// Initialize MongoDB connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to MongoDB!")

	// Initialize handlers
	authHandler := authentication.NewHandler(client, jwtSecret)

	// Initialize Gin router
	r := gin.Default()

	// Public routes
	r.POST("/api/login", authHandler.HandleLogin)
	r.POST("/api/signin", authHandler.HandleLogin)

	r.POST("/api/signup", authHandler.HandleSignup)

	// Login by token
	userAuthen := users.NewHandler(client, jwtSecret)
	r.POST("/api/user", userAuthen.HandleLoginByToken)

	// Protected routes
	auth := r.Group("/api")
	auth.Use(authHandler.AuthMiddleware())
	{

		expenseHandler := expense.NewHandler(client, jwtSecret)

		// Expense routes

		auth.POST("/expenses", expenseHandler.HandleCreateExpense)
		auth.GET("/expenses", expenseHandler.HandleGetExpenses)
		auth.GET("/expenses/:id", expenseHandler.HandleGetExpense)
		auth.PUT("/expenses/:id", expenseHandler.HandleUpdateExpense)
		auth.DELETE("/expenses/:id", expenseHandler.HandleDeleteExpense)
	}

	// Start server
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}

}
