package expense

type Expense struct {
	ID           string  `bson:"_id,omitempty"  json:"id,omitempty"`
	UserID       string  `bson:"user_id" json:"user_id"`
	CategoryID   string  `bson:"category_id,omitempty" json:"category_id,omitempty"`
	Amount       float64 `bson:"amount" json:"amount"`
	CurrencyCode string  `bson:"currency_code" json:"currency_code"`
	Name         string  `bson:"name" json:"name"`
	Description  string  `bson:"description" json:"description"`
	Date         string  `bson:"date" json:"date"`
}

type CreateExpenseRequest struct {
	Amount       float64 `json:"amount" binding:"required"`
	CategoryID   string  `json:"category_id,omitempty"`
	CurrencyCode string  `json:"currency_code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Description  string  `json:"description"`
	Date         string  `json:"date"`
}

type UpdateExpenseRequest struct {
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	CategoryID   string  `bson:"category_id,omitempty" json:"category_id,omitempty"`
}

// PaginatedExpenseResponse represents the paginated response for expenses
type PaginatedExpenseResponse struct {
	Expenses    []Expense `json:"expenses"`
	TotalCount  int64     `json:"total_count"`
	CurrentPage int       `json:"current_page"`
	TotalPages  int       `json:"total_pages"`
	Limit       int       `json:"limit"`
}

type GetLastExpensesResponse struct {
	TotalExpensesLast30Days float64 `json:"total_expenses_last_30_days"`
	TotalExpensesLast7Days  float64 `json:"total_expenses_last_7_days"`
}

// PaginatedExpenseResponse represents the paginated response for expenses
type GetMontlyExpensesResponse struct {
	Expenses    []Expense `json:"expenses"`
	TotalAmount int64     `json:"total_amount"`
}

type CSVUploadResponse struct {
	SuccessCount int      `json:"success_count"`
	ErrorCount   int      `json:"error_count"`
	Errors       []string `json:"errors,omitempty"`
}

type CSVExpense struct {
	Date  string // MM/dd/YYYY
	Name  string
	Price float64
	Note  string
}

type CSVExportExpense struct {
	Date         string
	Name         string
	Amount       float64
	CurrencyCode string
	Description  string
	CategoryID   string
	CategoryName string
}
