package expense

import (
	"context"
	"fmt"
	"math"
	"my-finance-backend/config"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Handler struct {
	mongoClient *mongo.Client
	jwtSecret   []byte
	config      *config.Config
}

func NewHandler(mongoClient *mongo.Client, config *config.Config, jwtSecret []byte) *Handler {
	return &Handler{
		mongoClient: mongoClient,
		jwtSecret:   jwtSecret,
		config:      config,
	}
}

// Create expense
func (h *Handler) HandleCreateExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	req.Date = time.Now().Format("2006-01-02")

	expense := Expense{
		ID:           primitive.NewObjectID().Hex(),
		UserID:       userID,
		Amount:       req.Amount,
		CurrencyCode: req.CurrencyCode,
		Name:         req.Name,
		Description:  req.Description,
		Date:         req.Date,
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, expense)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create expense"})
		return
	}

	c.JSON(http.StatusCreated, expense)
}

// Get all expenses for user with pagination
func (h *Handler) HandleGetExpenses(c *gin.Context) {
	userID := c.GetString("user_id")

	// Parse pagination parameters
	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err != nil || offset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset parameter"})
			return
		}
	}

	limit := 10 // default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
			return
		}
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("expenses")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create filter for user's expenses
	filter := bson.M{"user_id": userID}

	// Get total count of expenses
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not count expenses"})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	currentPage := (offset / limit) + 1

	// Find expenses with pagination
	findOptions := options.Find().
		SetSkip(int64(offset)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "date", Value: -1}}) // Sort by date descending

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch expenses"})
		return
	}
	defer cursor.Close(ctx)

	var expenses []Expense = make([]Expense, 0)
	if err = cursor.All(ctx, &expenses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode expenses"})
		return
	}

	response := PaginatedExpenseResponse{
		Expenses:    expenses,
		TotalCount:  totalCount,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		Limit:       limit,
	}
	c.JSON(http.StatusOK, response)
}

// Get single expense
func (h *Handler) HandleGetExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	expenseID := c.Param("id")

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var expense Expense
	err := collection.FindOne(ctx, bson.M{
		"_id":     expenseID,
		"user_id": userID,
	}).Decode(&expense)

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch expense"})
		return
	}

	c.JSON(http.StatusOK, expense)
}

// Update expense
func (h *Handler) HandleUpdateExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	expenseID := c.Param("id")

	var req UpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build update document
	update := bson.M{}
	if req.Expense != 0 {
		update["expense"] = req.Expense
	}
	if req.CurrencyCode != "" {
		update["currency_code"] = req.CurrencyCode
	}
	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Description != "" {
		update["description"] = req.Description
	}

	result, err := collection.UpdateOne(
		ctx,
		bson.M{
			"_id":     expenseID,
			"user_id": userID,
		},
		bson.M{"$set": update},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update expense"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
		return
	}

	// Get updated expense
	var expense Expense
	err = collection.FindOne(ctx, bson.M{
		"_id":     expenseID,
		"user_id": userID,
	}).Decode(&expense)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch updated expense"})
		return
	}

	c.JSON(http.StatusOK, expense)
}

// Delete expense
func (h *Handler) HandleDeleteExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	expenseID := c.Param("id")

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.DeleteOne(ctx, bson.M{
		"_id":     expenseID,
		"user_id": userID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete expense"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Expense deleted successfully"})
}
