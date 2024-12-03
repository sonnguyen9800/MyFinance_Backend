package expense

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct {
	mongoClient *mongo.Client
	jwtSecret   []byte
}

func NewHandler(mongoClient *mongo.Client, jwtSecret []byte) *Handler {
	return &Handler{
		mongoClient: mongoClient,
		jwtSecret:   jwtSecret,
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

	expense := Expense{
		ID:           primitive.NewObjectID().Hex(),
		UserID:       userID,
		Amount:       req.Amount,
		CurrencyCode: req.CurrencyCode,
		Name:         req.Name,
		Description:  req.Description,
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("payments")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, expense)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create expense"})
		return
	}

	c.JSON(http.StatusCreated, expense)
}

// Get all expenses for user
func (h *Handler) HandleGetExpenses(c *gin.Context) {
	userID := c.GetString("user_id")
	collection := h.mongoClient.Database("MyFinance_Dev").Collection("expenses")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch expenses"})
		return
	}
	defer cursor.Close(ctx)

	var expenses []Expense
	if err = cursor.All(ctx, &expenses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode expenses"})
		return
	}

	c.JSON(http.StatusOK, expenses)
}

// Get single expense
func (h *Handler) HandleGetExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	expenseID := c.Param("id")

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("expenses")
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

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("expenses")
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

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("expenses")
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
