package expense

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"my-finance-backend/apperr"
	"my-finance-backend/category"
	"my-finance-backend/config"
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
	repo        *Repository
}

func NewHandler(mongoClient *mongo.Client, config *config.Config, jwtSecret []byte) *Handler {
	return &Handler{
		mongoClient: mongoClient,
		jwtSecret:   jwtSecret,
		config:      config,
		repo:        NewRepository(mongoClient.Database(config.DatabaseName), config.CollectionExpensesName, config.CollectionCategoriesName),
	}
}

func (h *Handler) HandleGetLastExpenses(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	last7, _ := h.repo.SumSince(ctx, userID, 7)
	last30, _ := h.repo.SumSince(ctx, userID, 30)
	c.JSON(http.StatusOK, GetLastExpensesResponse{
		TotalExpensesLast7Days:  last7,
		TotalExpensesLast30Days: last30,
	})
}

func (h *Handler) HandleCreateExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if req.CategoryID != "" {
		ok, err := h.repo.CategoryExists(ctx, req.CategoryID)
		if err != nil {
			apperr.Respond(c, apperr.BadRequest("Invalid category ID"))
			return
		}
		if !ok {
			apperr.Respond(c, apperr.NotFound("Category not found"))
			return
		}
	}

	e := &Expense{
		UserID:        userID,
		CategoryID:    req.CategoryID,
		Amount:        req.Amount,
		CurrencyCode:  req.CurrencyCode,
		Name:          req.Name,
		Description:   req.Description,
		PaymentMethod: req.PaymentMethod,
		TagIDs:        req.TagIDs,
		Date:          req.Date,
	}
	if err := h.repo.Create(ctx, e); err != nil {
		apperr.Respond(c, apperr.Internal("Could not create expense"))
		return
	}
	c.JSON(http.StatusCreated, e)
}

func (h *Handler) HandleGetExpensesMonthly(c *gin.Context) {
	userID := c.GetString("user_id")

	month, year := 0, 0
	if s := c.Query("month"); s != "" {
		if _, err := fmt.Sscanf(s, "%d", &month); err != nil || month < 0 {
			apperr.Respond(c, apperr.BadRequest("Invalid month parameter"))
			return
		}
	}
	if s := c.Query("year"); s != "" {
		if _, err := fmt.Sscanf(s, "%d", &year); err != nil || year < 0 {
			apperr.Respond(c, apperr.BadRequest("Invalid year parameter"))
			return
		}
	}
	if month < 1 || month > 12 {
		apperr.Respond(c, apperr.BadRequest("Month must be between 1 and 12"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expenses, total, err := h.repo.Monthly(ctx, userID, month, year)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch expenses"))
		return
	}
	c.JSON(http.StatusOK, GetMontlyExpensesResponse{Expenses: expenses, TotalAmount: int64(total)})
}

func (h *Handler) HandleGetExpenses(c *gin.Context) {
	userID := c.GetString("user_id")

	offset := 0
	if s := c.Query("offset"); s != "" {
		if _, err := fmt.Sscanf(s, "%d", &offset); err != nil || offset < 0 {
			apperr.Respond(c, apperr.BadRequest("Invalid offset parameter"))
			return
		}
	}
	limit := 10
	if s := c.Query("limit"); s != "" {
		if _, err := fmt.Sscanf(s, "%d", &limit); err != nil || limit <= 0 {
			apperr.Respond(c, apperr.BadRequest("Invalid limit parameter"))
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expenses, total, err := h.repo.List(ctx, ListQuery{
		UserID:        userID,
		Offset:        offset,
		Limit:         limit,
		Search:        strings.TrimSpace(c.Query("search")),
		CategoryID:    c.Query("category_id"),
		TagID:         strings.TrimSpace(c.Query("tag_id")),
		PaymentMethod: strings.TrimSpace(c.Query("payment_method")),
		From:          strings.TrimSpace(c.Query("from")),
		To:            strings.TrimSpace(c.Query("to")),
	})
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch expenses"))
		return
	}

	c.JSON(http.StatusOK, PaginatedExpenseResponse{
		Expenses:    expenses,
		TotalCount:  total,
		CurrentPage: (offset / limit) + 1,
		TotalPages:  int(math.Ceil(float64(total) / float64(limit))),
		Limit:       limit,
	})
}

func (h *Handler) HandleGetExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid expense ID"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	e, err := h.repo.GetByID(ctx, userID, id)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Expense not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch expense"))
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *Handler) HandleUpdateExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid expense ID"))
		return
	}

	var req UpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	if req.PaymentMethod != "" {
		update["payment_method"] = req.PaymentMethod
	}
	if req.TagIDs != nil {
		update["tag_ids"] = req.TagIDs
	}
	if req.CategoryID != "" {
		ok, err := h.repo.CategoryExists(ctx, req.CategoryID)
		if err != nil {
			apperr.Respond(c, apperr.BadRequest("Invalid category ID"))
			return
		}
		if !ok {
			apperr.Respond(c, apperr.NotFound("Category not found"))
			return
		}
		update["category_id"] = req.CategoryID
	}

	e, err := h.repo.Update(ctx, userID, id, update)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Expense not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not update expense"))
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *Handler) HandleDeleteExpense(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid expense ID"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.repo.Delete(ctx, userID, id)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Expense not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not delete expense"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Expense deleted successfully"})
}

// ── CSV import/export ───────────────────────────────────────────────────
// These retain direct collection access for now (I/O-heavy); migrating them
// onto the repository is a documented follow-up.

func (h *Handler) HandleUploadCSV(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("No file uploaded"))
		return
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".csv") {
		apperr.Respond(c, apperr.BadRequest("File must be a CSV"))
		return
	}

	src, err := file.Open()
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not open file"))
		return
	}

	reader := csv.NewReader(src)
	reader.FieldsPerRecord = 5
	reader.TrimLeadingSpace = true

	if _, err = reader.Read(); err != nil {
		apperr.Respond(c, apperr.BadRequest("Could not read CSV header"))
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var response CSVUploadResponse
	var errs []string
	lineCount := 2
	var currentDate time.Time

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("Line %d: Could not read row", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}

		dateStr := strings.TrimSpace(record[0])
		var date time.Time
		if dateStr == "" {
			if currentDate.IsZero() {
				errs = append(errs, fmt.Sprintf("Line %d: Empty date field with no previous valid date", lineCount))
				response.ErrorCount++
				lineCount++
				continue
			}
			date = currentDate
		} else {
			date, err = time.Parse("1/2/2006", dateStr)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Line %d: Invalid date format", lineCount))
				response.ErrorCount++
				lineCount++
				continue
			}
			currentDate = date
		}

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
			errs = append(errs, fmt.Sprintf("Line %d: Invalid price", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}
		currency := strings.TrimSpace(record[4])
		if currency == "" {
			currency = "VND"
		}
		if currency == "VND" {
			price = price * 1000
		}

		expense := Expense{
			UserID:       userID,
			Amount:       price,
			CurrencyCode: currency,
			Name:         name,
			Description:  strings.TrimSpace(record[3]),
			Date:         date.Format("2006-01-02"),
		}

		existing := Expense{}
		if err := collection.FindOne(ctx, bson.M{"user_id": userID, "name": expense.Name, "date": expense.Date}).Decode(&existing); err == nil {
			errs = append(errs, fmt.Sprintf("Line %d: Expense with same name and date existed", lineCount))
			response.ErrorCount++
			lineCount++
			continue
		}

		if _, err := collection.InsertOne(ctx, expense); err != nil {
			errs = append(errs, fmt.Sprintf("Line %d: Could not save expense", lineCount))
			response.ErrorCount++
		} else {
			response.SuccessCount++
		}
		lineCount++
	}

	response.Errors = errs
	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleDownloadCSV(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionExpensesName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	findOptions := options.Find().SetSort(bson.D{{Key: "date", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID}, findOptions)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch expenses"))
		return
	}
	defer cursor.Close(ctx)

	var expenses []Expense
	if err = cursor.All(ctx, &expenses); err != nil {
		apperr.Respond(c, apperr.Internal("Could not decode expenses"))
		return
	}

	categoryCollection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	categoryMap := make(map[string]string)
	for _, expense := range expenses {
		if expense.CategoryID != "" {
			if _, exists := categoryMap[expense.CategoryID]; !exists {
				var cat category.Category
				if err := categoryCollection.FindOne(ctx, bson.M{"_id": expense.CategoryID}).Decode(&cat); err == nil {
					categoryMap[expense.CategoryID] = cat.Name
				}
			}
		}
	}

	buf := new(bytes.Buffer)
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(buf)

	header := []string{"Date", "Name", "Amount", "CurrencyCode", "Description", "CategoryID", "Category"}
	if err := writer.Write(header); err != nil {
		apperr.Respond(c, apperr.Internal("Could not write CSV header"))
		return
	}

	for _, expense := range expenses {
		categoryName := ""
		if expense.CategoryID != "" {
			categoryName = categoryMap[expense.CategoryID]
		}
		if date, err := time.Parse("2006-01-02", expense.Date); err == nil {
			expense.Date = date.Format("1/2/2006")
		}
		row := []string{
			expense.Date, expense.Name, fmt.Sprintf("%.2f", expense.Amount),
			expense.CurrencyCode, expense.Description, expense.CategoryID, categoryName,
		}
		if err := writer.Write(row); err != nil {
			apperr.Respond(c, apperr.Internal("Could not write CSV row"))
			return
		}
	}
	writer.Flush()

	filename := fmt.Sprintf("expenses_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}
