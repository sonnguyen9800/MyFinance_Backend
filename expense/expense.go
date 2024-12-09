package expense

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"my-finance-backend/category"
	"my-finance-backend/config"
	"my-finance-backend/utils"
	"net/http"
	"strconv"
	"strings"
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

	result, err := collection.InsertOne(ctx, expense)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create expense"})
		return
	}
	expense.ID = result.InsertedID.(primitive.ObjectID).Hex()

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
	objectId, error := utils.StringToObjectId(expenseID)

	if error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expense ID " + error.Error()})
		return
	}

	var expense Expense
	err := collection.FindOne(ctx, bson.M{
		"_id":     objectId,
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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
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
	if req.Date != "" {
		update["date"] = req.Date
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

	objectId, error := utils.StringToObjectId(expenseID)
	if error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expense ID " + error.Error()})
		return
	}
	result, err := collection.UpdateOne(
		ctx,
		bson.M{
			"_id":     objectId,
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
		"_id":     objectId,
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
	objectId, error := utils.StringToObjectId(expenseID)
	if error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expense ID " + error.Error()})
		return
	}
	result, err := collection.DeleteOne(ctx, bson.M{
		"_id":     objectId,
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

// HandleUploadCSV handles the upload of expenses via CSV file
func (h *Handler) HandleUploadCSV(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// Get the file from the request
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Check file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".csv") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be a CSV"})
		return
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not open file"})
		return
	}
	// defer src.Close()

	// Create CSV reader
	reader := csv.NewReader(src)
	reader.FieldsPerRecord = 5 // Expecting 4 fields: Date, Name, Price, Note
	reader.TrimLeadingSpace = true

	// Skip header if exists
	_, err = reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read CSV header"})
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var response CSVUploadResponse
	var errors []string

	// Process each row
	lineCount := 2            // Start from line 2 (after header)
	var currentDate time.Time // Keep track of the current date for empty date fields

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: Could not read row", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}

		// Clean and process date
		dateStr := strings.TrimSpace(record[0])
		var date time.Time

		if dateStr == "" {
			// If date is empty, use the previous date
			if currentDate.IsZero() {
				errors = append(errors, fmt.Sprintf("Line %d: Empty date field with no previous valid date", lineCount))
				response.ErrorCount++
				lineCount++
				continue
			}
			date = currentDate
		} else {
			// Parse date (MM/dd/YYYY)
			date, err = time.Parse("1/2/2006", dateStr)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Line %d: Invalid date format", lineCount))
				response.ErrorCount++
				lineCount++
				continue
			}
			currentDate = date // Update current date for next empty date field
		}

		// Parse price and multiply by 1000
		priceStr := strings.TrimSpace(record[2])
		price, err := strconv.ParseFloat(priceStr, 64)

		name := strings.TrimSpace(record[1])

		if name == "" && priceStr == "" {
			lineCount++
			continue
		}
		if name == "" {
			name = "No Name"
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: Invalid price", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}
		currency := strings.TrimSpace(record[4])
		if currency == "" {
			currency = "VND"
		}
		if currency == "VND" {
			price = price * 1000 // Multiply by 1000 as per requirement
		}
		// Create expense

		expense := Expense{
			UserID:       userID,
			Amount:       price,
			CurrencyCode: currency,
			Name:         name,
			Description:  strings.TrimSpace(record[3]),
			Date:         date.Format("2006-01-02"),
		}
		// Check if expense with same name and date existed
		existingExpense := Expense{}
		err = collection.FindOne(ctx, bson.M{
			"user_id": userID,
			"name":    expense.Name,
			"date":    expense.Date,
		}).Decode(&existingExpense)
		if err == nil {
			errors = append(errors, fmt.Sprintf("Line %d: Expense with same name and date existed", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}

		// Insert expense
		_, err = collection.InsertOne(ctx, expense)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: Could not save expense", lineCount))
			response.ErrorCount++
		} else {
			response.SuccessCount++
		}

		lineCount++
	}

	response.Errors = errors
	c.JSON(http.StatusOK, response)
}

// HandleDownloadCSV handles the download of all expenses in CSV format
func (h *Handler) HandleDownloadCSV(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// Get all expenses for the user
	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sort by date
	findOptions := options.Find().SetSort(bson.D{{Key: "date", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID}, findOptions)
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

	// Create a map of category IDs to names
	categoryCollection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	categoryMap := make(map[string]string)

	for _, expense := range expenses {
		if expense.CategoryID != "" {
			if _, exists := categoryMap[expense.CategoryID]; !exists {
				var cat category.Category
				err := categoryCollection.FindOne(ctx, bson.M{"_id": expense.CategoryID}).Decode(&cat)
				if err == nil {
					categoryMap[expense.CategoryID] = cat.Name
				}
			}
		}
	}

	// Create CSV buffer with UTF-8 BOM
	buf := new(bytes.Buffer)

	// Write UTF-8 BOM
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(buf)

	// Write header
	header := []string{"Date", "Name", "Amount", "CurrencyCode", "Description", "CategoryID", "Category"}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not write CSV header"})
		return
	}

	// Write data
	for _, expense := range expenses {
		// Get category name if exists
		categoryName := ""
		if expense.CategoryID != "" {
			categoryName = categoryMap[expense.CategoryID]
		}

		// Format date from YYYY-MM-DD to MM/dd/YYYY
		date, err := time.Parse("2006-01-02", expense.Date)
		if err == nil {
			expense.Date = date.Format("1/2/2006")
		}

		row := []string{
			expense.Date,
			expense.Name,
			fmt.Sprintf("%.2f", expense.Amount), // Keep original amount (already x1000)
			expense.CurrencyCode,
			expense.Description,
			expense.CategoryID,
			categoryName,
		}

		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not write CSV row"})
			return
		}
	}

	writer.Flush()

	// Set headers for file download
	currentTime := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("expenses_%s.csv", currentTime)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}
