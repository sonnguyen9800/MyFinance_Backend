package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"my-finance-backend/authentication"
	"my-finance-backend/category"
	"my-finance-backend/expense"
	"my-finance-backend/portfolio"
	"my-finance-backend/tag"
	"my-finance-backend/version"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// authMiddleware validates the JWT on protected routes.
func authMiddleware(secret []byte) gin.HandlerFunc {
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
			return secret, nil
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
	config := LoadConfig()

	// Security: fail fast on an unsafe JWT secret in production
	if config.JWTSecret == "" || config.JWTSecret == "your-dev-secret-key" {
		if config.AppEnv == "production" || config.AppEnv == "prod" {
			log.Fatal("FATAL: JWT_SECRET must be set to a strong, unique value in production")
		}
		slog.Warn("Using insecure default JWT secret — set JWT_SECRET before deploying to production")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.DatabaseURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	slog.Info("Connected to MongoDB", "env", config.AppEnv, "database", config.DatabaseName)

	if err := ensureIndexes(client, config); err != nil {
		slog.Error("Could not create database indexes", "error", err)
	}

	authHandler := authentication.NewHandler(client, config, []byte(config.JWTSecret))
	expenseHandler := expense.NewHandler(client, config, []byte(config.JWTSecret))
	categoryHandler := category.NewHandler(client, config, []byte(config.JWTSecret))
	tagHandler := tag.NewHandler(client, config)
	portfolioHandler := portfolio.NewHandler(client, config)

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	loginLimiter := newRateLimiter(5, time.Minute)

	healthHandler := func(c *gin.Context) {
		info := version.GetInfo()
		info.GoVersion = runtime.Version()
		info.ServerEnv = config.AppEnv
		info.DatabaseName = config.DatabaseName
		c.JSON(http.StatusOK, info)
	}
	r.GET("/api/ping", healthHandler)
	r.GET("/api/test", healthHandler)
	r.GET("/api/health", healthHandler)

	r.POST("/api/login", loginLimiter, authHandler.HandleLogin)
	r.POST("/api/signin", loginLimiter, authHandler.HandleLogin)
	r.POST("/api/signup", loginLimiter, authHandler.HandleSignup)
	r.POST("/api/auth/google", loginLimiter, authHandler.HandleGoogleAuth)
	r.POST("/api/user", authHandler.HandleLoginByToken)

	auth := r.Group("/api")
	auth.Use(authMiddleware([]byte(config.JWTSecret)))
	{
		// Account management
		auth.POST("/auth/refresh", authHandler.HandleRefreshToken)
		auth.PUT("/user", authHandler.HandleUpdateUser)
		auth.PUT("/user/password", authHandler.HandleChangePassword)
		auth.DELETE("/user", authHandler.HandleDeleteAccount)

		// Category routes
		auth.POST("/categories", categoryHandler.HandleCreateCategory)
		auth.GET("/categories", categoryHandler.HandleGetCategories)
		auth.GET("/categories/:id", categoryHandler.HandleGetCategory)
		auth.PUT("/categories/:id", categoryHandler.HandleUpdateCategory)
		auth.DELETE("/categories/:id", categoryHandler.HandleDeleteCategory)

		// Tag routes (authenticated + full CRUD)
		auth.POST("/tags", tagHandler.HandleCreateTag)
		auth.GET("/tags", tagHandler.HandleGetTags)
		auth.GET("/tags/:id", tagHandler.HandleGetTag)
		auth.PUT("/tags/:id", tagHandler.HandleUpdateTag)
		auth.DELETE("/tags/:id", tagHandler.HandleDeleteTag)

		// Portfolio asset class routes
		auth.POST("/asset_classes", portfolioHandler.HandleCreateAssetClass)
		auth.GET("/asset_classes", portfolioHandler.HandleGetAssetClasses)

		// Portfolio asset routes
		auth.POST("/assets", portfolioHandler.HandleCreateAsset)
		auth.GET("/assets", portfolioHandler.HandleGetAssets)
		auth.GET("/assets/:id", portfolioHandler.HandleGetAsset)
		auth.PATCH("/assets/:id", portfolioHandler.HandleUpdateAsset)
		auth.DELETE("/assets/:id", portfolioHandler.HandleDeleteAsset)

		// Position lots routes
		auth.POST("/assets/:id/lots", portfolioHandler.HandleCreatePositionLot)
		auth.GET("/assets/:id/lots", portfolioHandler.HandleGetPositionLots)
		auth.PATCH("/assets/:id/lots/:lotId", portfolioHandler.HandleUpdatePositionLot)
		auth.DELETE("/assets/:id/lots/:lotId", portfolioHandler.HandleDeletePositionLot)

		// Cash flow routes
		auth.POST("/assets/:id/cashflows", portfolioHandler.HandleCreateCashFlow)
		auth.GET("/assets/:id/cashflows", portfolioHandler.HandleGetCashFlows)
		auth.DELETE("/assets/:id/cashflows/:flowId", portfolioHandler.HandleDeleteCashFlow)

		// Valuation routes
		auth.POST("/assets/:id/valuations", portfolioHandler.HandleCreateValuationSnapshot)
		auth.GET("/assets/:id/valuations", portfolioHandler.HandleGetValuationSnapshots)
		auth.DELETE("/assets/:id/valuations/:valuationId", portfolioHandler.HandleDeleteValuationSnapshot)

		// FX rate management
		auth.POST("/fx_rates", portfolioHandler.HandleCreateFxRate)
		auth.GET("/fx_rates", portfolioHandler.HandleGetFxRates)
		auth.DELETE("/fx_rates/:rateId", portfolioHandler.HandleDeleteFxRate)

		// Portfolio summary
		auth.GET("/portfolio/summary", portfolioHandler.HandleGetPortfolioSummary)

		// Expense routes
		auth.POST("/expenses", expenseHandler.HandleCreateExpense)
		auth.GET("/expenses", expenseHandler.HandleGetExpenses)
		auth.GET("/expenses_last", expenseHandler.HandleGetLastExpenses)
		auth.GET("/expenses_montly", expenseHandler.HandleGetExpensesMonthly)
		auth.POST("/expenses/upload", expenseHandler.HandleUploadCSV)
		auth.GET("/expenses/download", expenseHandler.HandleDownloadCSV)
		auth.GET("/expenses/:id", expenseHandler.HandleGetExpense)
		auth.PUT("/expenses/:id", expenseHandler.HandleUpdateExpense)
		auth.DELETE("/expenses/:id", expenseHandler.HandleDeleteExpense)
	}

	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		slog.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server…")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	if err := client.Disconnect(shutdownCtx); err != nil {
		slog.Error("Error disconnecting MongoDB", "error", err)
	}
	slog.Info("Server exited cleanly")
}
