package portfolio

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Asset struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	UserID          string             `bson:"user_id"`
	AssetClassCode  string             `bson:"asset_class_code"`
	DisplayName     string             `bson:"display_name"`
	DefaultCurrency string             `bson:"default_currency"`
	Metadata        map[string]any     `bson:"metadata,omitempty"`
	Active          bool               `bson:"active"`
	CreatedAt       time.Time          `bson:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at"`
	Tags            []string           `bson:"tags,omitempty"`
	Attributes      map[string]string  `bson:"attributes,omitempty"`
	Notes           string             `bson:"notes,omitempty"`
	ExpectedReturn  *float64           `bson:"expected_return,omitempty"`
}

type CreateAssetRequest struct {
	DisplayName     string            `json:"display_name" binding:"required"`
	AssetClassCode  string            `json:"asset_class_code" binding:"required"`
	DefaultCurrency string            `json:"default_currency" binding:"required"`
	Metadata        map[string]any    `json:"metadata"`
	Tags            []string          `json:"tags"`
	Attributes      map[string]string `json:"attributes"`
	Notes           string            `json:"notes"`
	ExpectedReturn  *float64          `json:"expected_return"`
	Active          *bool             `json:"active"`
}

type UpdateAssetRequest struct {
	DisplayName     *string           `json:"display_name"`
	DefaultCurrency *string           `json:"default_currency"`
	Metadata        map[string]any    `json:"metadata"`
	Tags            []string          `json:"tags"`
	Attributes      map[string]string `json:"attributes"`
	Notes           *string           `json:"notes"`
	ExpectedReturn  *float64          `json:"expected_return"`
	Active          *bool             `json:"active"`
}

type AssetResponse struct {
	ID              string            `json:"id"`
	AssetClassCode  string            `json:"asset_class_code"`
	DisplayName     string            `json:"display_name"`
	DefaultCurrency string            `json:"default_currency"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	ExpectedReturn  *float64          `json:"expected_return,omitempty"`
	Active          bool              `json:"active"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type AssetClass struct {
	ID                    primitive.ObjectID `bson:"_id,omitempty"`
	Code                  string             `bson:"code"`
	Name                  string             `bson:"name"`
	DefaultValuationModel string             `bson:"default_valuation_method"`
	MetadataSchema        map[string]any     `bson:"metadata_schema,omitempty"`
	CreatedAt             time.Time          `bson:"created_at"`
	UpdatedAt             time.Time          `bson:"updated_at"`
}

type CreateAssetClassRequest struct {
	Code                  string         `json:"code" binding:"required"`
	Name                  string         `json:"name" binding:"required"`
	DefaultValuationModel string         `json:"default_valuation_method"`
	MetadataSchema        map[string]any `json:"metadata_schema"`
}

type AssetClassResponse struct {
	Code                  string         `json:"code"`
	Name                  string         `json:"name"`
	DefaultValuationModel string         `json:"default_valuation_method,omitempty"`
	MetadataSchema        map[string]any `json:"metadata_schema,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type PositionLot struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	AssetID      primitive.ObjectID `bson:"asset_id"`
	UserID       string             `bson:"user_id"`
	AcquiredAt   time.Time          `bson:"acquired_at"`
	Quantity     float64            `bson:"quantity"`
	UnitCost     float64            `bson:"unit_cost"`
	CostCurrency string             `bson:"cost_currency"`
	Fees         float64            `bson:"fees"`
	Notes        string             `bson:"notes,omitempty"`
	FxPair       string             `bson:"fx_pair,omitempty"`
	FxRateUsed   *float64           `bson:"fx_rate_used,omitempty"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

type CreatePositionLotRequest struct {
	AcquiredAt   string   `json:"acquired_at"`
	Quantity     float64  `json:"quantity" binding:"required"`
	UnitCost     float64  `json:"unit_cost" binding:"required"`
	CostCurrency string   `json:"cost_currency" binding:"required"`
	Fees         float64  `json:"fees"`
	Notes        string   `json:"notes"`
	FxPair       string   `json:"fx_pair"`
	FxRateUsed   *float64 `json:"fx_rate_used"`
}

type UpdatePositionLotRequest struct {
	AcquiredAt   *string  `json:"acquired_at"`
	Quantity     *float64 `json:"quantity"`
	UnitCost     *float64 `json:"unit_cost"`
	CostCurrency *string  `json:"cost_currency"`
	Fees         *float64 `json:"fees"`
	Notes        *string  `json:"notes"`
	FxPair       *string  `json:"fx_pair"`
	FxRateUsed   *float64 `json:"fx_rate_used"`
}

type PositionLotResponse struct {
	ID           string    `json:"id"`
	AssetID      string    `json:"asset_id"`
	Quantity     float64   `json:"quantity"`
	UnitCost     float64   `json:"unit_cost"`
	CostCurrency string    `json:"cost_currency"`
	Fees         float64   `json:"fees"`
	Notes        string    `json:"notes,omitempty"`
	FxPair       string    `json:"fx_pair,omitempty"`
	FxRateUsed   *float64  `json:"fx_rate_used,omitempty"`
	AcquiredAt   time.Time `json:"acquired_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CashFlowType string

const (
	CashFlowDeposit    CashFlowType = "DEPOSIT"
	CashFlowWithdrawal CashFlowType = "WITHDRAWAL"
	CashFlowDividend   CashFlowType = "DIVIDEND"
	CashFlowInterest   CashFlowType = "INTEREST"
)

type CashFlow struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	AssetID    primitive.ObjectID `bson:"asset_id"`
	UserID     string             `bson:"user_id"`
	Type       CashFlowType       `bson:"type"`
	Amount     float64            `bson:"amount"`
	Currency   string             `bson:"currency"`
	OccurredAt time.Time          `bson:"occurred_at"`
	Notes      string             `bson:"notes,omitempty"`
	CreatedAt  time.Time          `bson:"created_at"`
}

type CreateCashFlowRequest struct {
	Type       CashFlowType `json:"type" binding:"required"`
	Amount     float64      `json:"amount" binding:"required"`
	Currency   string       `json:"currency" binding:"required"`
	OccurredAt string       `json:"occurred_at"`
	Notes      string       `json:"notes"`
}

type CashFlowResponse struct {
	ID         string       `json:"id"`
	AssetID    string       `json:"asset_id"`
	Type       CashFlowType `json:"type"`
	Amount     float64      `json:"amount"`
	Currency   string       `json:"currency"`
	OccurredAt time.Time    `json:"occurred_at"`
	Notes      string       `json:"notes,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

type ValuationSnapshot struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty"`
	AssetID              primitive.ObjectID `bson:"asset_id"`
	UserID               string             `bson:"user_id"`
	CapturedAt           time.Time          `bson:"captured_at"`
	NativeCurrencyValue  float64            `bson:"native_currency_value"`
	NativeCurrency       string             `bson:"native_currency"`
	ValuationCurrencyMap map[string]float64 `bson:"valuation_currency_map,omitempty"`
	RatesUsed            map[string]float64 `bson:"rates_used,omitempty"`
	Notes                string             `bson:"notes,omitempty"`
	CreatedAt            time.Time          `bson:"created_at"`
	Source               string             `bson:"source,omitempty"`
}

type CreateValuationSnapshotRequest struct {
	CapturedAt           string             `json:"captured_at"`
	NativeCurrencyValue  float64            `json:"native_currency_value" binding:"required"`
	NativeCurrency       string             `json:"native_currency" binding:"required"`
	ValuationCurrencyMap map[string]float64 `json:"valuation_currency_map"`
	RatesUsed            map[string]float64 `json:"rates_used"`
	Notes                string             `json:"notes"`
	Source               string             `json:"source"`
}

type ValuationSnapshotResponse struct {
	ID                   string             `json:"id"`
	AssetID              string             `json:"asset_id"`
	CapturedAt           time.Time          `json:"captured_at"`
	NativeCurrencyValue  float64            `json:"native_currency_value"`
	NativeCurrency       string             `json:"native_currency"`
	ValuationCurrencyMap map[string]float64 `json:"valuation_currency_map,omitempty"`
	RatesUsed            map[string]float64 `json:"rates_used,omitempty"`
	Notes                string             `json:"notes,omitempty"`
	Source               string             `json:"source,omitempty"`
	CreatedAt            time.Time          `json:"created_at"`
}

type FxRate struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	UserID        string             `bson:"user_id,omitempty"`
	BaseCurrency  string             `bson:"base_currency"`
	QuoteCurrency string             `bson:"quote_currency"`
	Rate          float64            `bson:"rate"`
	EffectiveAt   time.Time          `bson:"effective_at"`
	Source        string             `bson:"source,omitempty"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type CreateFxRateRequest struct {
	BaseCurrency  string  `json:"base_currency" binding:"required"`
	QuoteCurrency string  `json:"quote_currency" binding:"required"`
	Rate          float64 `json:"rate" binding:"required"`
	EffectiveAt   string  `json:"effective_at"`
	Source        string  `json:"source"`
}

type FxRateResponse struct {
	ID            string    `json:"id"`
	BaseCurrency  string    `json:"base_currency"`
	QuoteCurrency string    `json:"quote_currency"`
	Rate          float64   `json:"rate"`
	EffectiveAt   time.Time `json:"effective_at"`
	Source        string    `json:"source,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type PortfolioPositionSummary struct {
	Asset                 AssetResponse `json:"asset"`
	CurrentValue          float64       `json:"current_value"`
	CostBasis             float64       `json:"cost_basis"`
	UnrealizedGain        float64       `json:"unrealized_gain"`
	UnrealizedGainPercent float64       `json:"unrealized_gain_percent"`
	LastValuationAt       *time.Time    `json:"last_valuation_at,omitempty"`
}

type PortfolioSummaryResponse struct {
	BaseCurrency        string                     `json:"base_currency"`
	TotalCurrentValue   float64                    `json:"total_current_value"`
	TotalCostBasis      float64                    `json:"total_cost_basis"`
	TotalUnrealizedGain float64                    `json:"total_unrealized_gain"`
	Positions           []PortfolioPositionSummary `json:"positions"`
	GeneratedAt         time.Time                  `json:"generated_at"`
}
