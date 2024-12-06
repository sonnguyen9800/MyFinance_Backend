package expense

import (
	"context"
	"fmt"
	"log"
	"math"
	"my-finance-backend/category"
	"my-finance-backend/config"
	"my-finance-backend/utils"
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

func (h *Handler) HandleGetLastExpenses(c *gin.Context) {
	userID := c.GetString("user_id")
	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	filter := bson.M{"user_id": userID}
	// Get total count of expenses
	_, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not count expenses"})
		return
	}

	calculateSum := func(limit int) float64 {

		pipeline := mongo.Pipeline{
			// Step 1: Group by unique dates
			// Step 1: Group by date and calculate total amount per date
			{{"$group", bson.D{
				{"_id", "$date"}, // Group by the "date" field
				{"dailyTotal", bson.D{{"$sum", "$amount"}}}, // Sum the "amount" field for each date
			}}},
			// Step 2: Sort the grouped dates in descending order
			{{"$sort", bson.D{{"_id", -1}}}},
			// Step 3: Limit to the latest 30 unique dates
			{{"$limit", limit}},
			// Step 4: Calculate the sum of all daily totals
			{{"$group", bson.D{
				{"_id", nil}, // No grouping key, combine all results
				{"totalValue", bson.D{{"$sum", "$dailyTotal"}}}, // Sum the daily totals
			}}},
		}
		//

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			log.Fatalf("Error executing aggregation for limit %d: %v", limit, err)
		}
		defer cursor.Close(ctx)

		// Retrieve the result
		var result []bson.M
		if err = cursor.All(ctx, &result); err != nil {
			log.Fatalf("Error decoding aggregation result for limit %d: %v", limit, err)
		}

		// Return the sum or 0 if no records are found
		if len(result) > 0 {
			if totalValue, ok := result[0]["totalValue"].(float64); ok {
				return totalValue
			}
		}
		return 0
	}

	var sumLast7 = calculateSum(7)
	var sumLast30 = calculateSum(30)

	GetTotalExpensesResponse := GetLastExpensesResponse{
		TotalExpensesLast30Days: sumLast30,
		TotalExpensesLast7Days:  sumLast7,
	}

	c.JSON(http.StatusOK, GetTotalExpensesResponse)
}

// Create expense
func (h *Handler) HandleCreateExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Set current date if not provided
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	expense := Expense{
		ID:           primitive.NewObjectID().Hex(),
		UserID:       userID,
		CategoryID:   req.CategoryID,
		Amount:       req.Amount,
		CurrencyCode: req.CurrencyCode,
		Name:         req.Name,
		Description:  req.Description,
		Date:         req.Date,
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	categoryID := req.CategoryID
	if categoryID != "" {
		collectionCategory := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)

		categoryObjectId, error := utils.StringToObjectId(categoryID)
		if error != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
			return
		}
		// Check if category exists
		filter := bson.M{"_id": categoryObjectId}
		var category category.Category
		err := collectionCategory.FindOne(ctx, filter).Decode(&category)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}

	}

	_, err := collection.InsertOne(ctx, expense)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create expense"})
		return
	}

	c.JSON(http.StatusCreated, expense)
}

func (h *Handler) HandleGetExpensesMonthly(c *gin.Context) {
	userID := c.GetString("user_id")

	month := 0
	year := 0
	if monthStr := c.Query("month"); monthStr != "" {
		if _, err := fmt.Sscanf(monthStr, "%d", &month); err != nil || month < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month parameter"})
			return
		}
	}

	if yearStr := c.Query("year"); yearStr != "" {
		if _, err := fmt.Sscanf(yearStr, "%d", &year); err != nil || year < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year parameter"})
			return
		}
	}

	// Validate month (1-12)
	if month < 1 || month > 13 {
		c.JSON(http.StatusBadRequest,
			gin.H{
				"error":         "Month must be between 1 and 12, current month: $month",
				"current month": month,
				"current year":  year,
			})
		return
	}
	month_string := time.Month(month)

	// Get first day of the month
	startDate := time.Date(year, month_string, 1, 0, 0, 0, 0, time.UTC)
	// Get first day of next month
	endDate := startDate.AddDate(0, 1, 0)

	// Format dates as strings (YYYY-MM-DD)
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Build filter for the date range and user
	filter := bson.M{
		"user_id": userID,
		"date": bson.M{
			"$gte": startDateStr,
			"$lt":  endDateStr,
		},
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get expenses for the month
	cursor, err := collection.Find(ctx, filter)
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

	// Calculate total amount
	var totalAmount float64
	for _, expense := range expenses {
		totalAmount += expense.Amount
	}

	response := GetMontlyExpensesResponse{
		Expenses:    expenses,
		TotalAmount: int64(totalAmount),
	}

	c.JSON(http.StatusOK, response)
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

	// Build filter
	filter := bson.M{"user_id": userID}

	// Add category filter if provided
	if categoryID := c.Query("category_id"); categoryID != "" {
		filter["category_id"] = categoryID
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total count of expenses
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not count expenses"})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	currentPage := (offset / limit) + 1

	// Get paginated expenses
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
	if req.Amount != 0 {
		update["amount"] = req.Amount
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

	categoryID := req.CategoryID
	if categoryID != "" {
		collectionCategory := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)

		categoryObjectId, error := utils.StringToObjectId(categoryID)
		if error != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
			return
		}
		// Check if category exists
		filter := bson.M{"_id": categoryObjectId}
		var category category.Category
		err := collectionCategory.FindOne(ctx, filter).Decode(&category)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		update["category_id"] = req.CategoryID

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
