package expense

type Expense struct {
	ID           string  `bson:"_id" json:"id"`
	UserID       string  `bson:"user_id" json:"user_id"`
	Amount       float64 `bson:"amount" json:"amount"`
	CurrencyCode string  `bson:"currency_code" json:"currency_code"`
	Name         string  `bson:"name" json:"name"`
	Description  string  `bson:"description" json:"description"`
	Date         string  `bson:"date" json:"date"`
	CategoryID   string  `bson:"category_id,omitempty" json:"category_id,omitempty"`
}

type CreateExpenseRequest struct {
	Amount       float64 `json:"amount" binding:"required"`
	CurrencyCode string  `json:"currency_code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Description  string  `json:"description"`
	Date         string  `json:"date"`
	CategoryID   string  `bson:"category_id,omitempty" json:"category_id,omitempty"`
}

type UpdateExpenseRequest struct {
	Expense      float64 `json:"expense"`
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
