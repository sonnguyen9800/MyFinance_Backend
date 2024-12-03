package expense

type Expense struct {
	ID           string  `bson:"_id" json:"id"`
	UserID       string  `bson:"user_id" json:"user_id"`
	Amount       float64 `bson:"amount" json:"amount"`
	CurrencyCode string  `bson:"currency_code" json:"currency_code"`
	Name         string  `bson:"name" json:"name"`
	Description  string  `bson:"description" json:"description"`
	Date         string  `bson:"date" json:"date"`
}

type CreateExpenseRequest struct {
	Amount       float64 `json:"amount" binding:"required"`
	CurrencyCode string  `json:"currency_code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Description  string  `json:"description"`
	Date         string  `json:"date"`
}

type UpdateExpenseRequest struct {
	Expense      float64 `json:"expense"`
	CurrencyCode string  `json:"currency_code"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
}
