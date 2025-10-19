package portfolio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-finance-backend/config"
	"my-finance-backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultMongoTimeout = 8 * time.Second
	timeLayoutRFC3339   = time.RFC3339
)

type Handler struct {
	mongoClient *mongo.Client
	config      *config.Config
}

func NewHandler(mongoClient *mongo.Client, cfg *config.Config) *Handler {
	return &Handler{
		mongoClient: mongoClient,
		config:      cfg,
	}
}

func (h *Handler) assetsCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionAssetsName)
}

func (h *Handler) assetClassesCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionAssetClassesName)
}

func (h *Handler) positionLotsCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionPositionLotsName)
}

func (h *Handler) cashFlowsCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCashFlowsName)
}

func (h *Handler) valuationsCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionValuationsName)
}

func (h *Handler) fxRatesCollection() *mongo.Collection {
	return h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionFxRatesName)
}

func (h *Handler) newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultMongoTimeout)
}

func parseTimeOrDefault(value string, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.Parse(timeLayoutRFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func formatTimeOrZero(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

func toObjectID(id string) (primitive.ObjectID, error) {
	return utils.StringToObjectId(id)
}

func assetToResponse(asset Asset) AssetResponse {
	return AssetResponse{
		ID:              asset.ID.Hex(),
		AssetClassCode:  asset.AssetClassCode,
		DisplayName:     asset.DisplayName,
		DefaultCurrency: asset.DefaultCurrency,
		Metadata:        asset.Metadata,
		Tags:            asset.Tags,
		Attributes:      asset.Attributes,
		Notes:           asset.Notes,
		ExpectedReturn:  asset.ExpectedReturn,
		Active:          asset.Active,
		CreatedAt:       formatTimeOrZero(asset.CreatedAt),
		UpdatedAt:       formatTimeOrZero(asset.UpdatedAt),
	}
}

func assetClassToResponse(class AssetClass) AssetClassResponse {
	return AssetClassResponse{
		Code:                  class.Code,
		Name:                  class.Name,
		DefaultValuationModel: class.DefaultValuationModel,
		MetadataSchema:        class.MetadataSchema,
		CreatedAt:             formatTimeOrZero(class.CreatedAt),
		UpdatedAt:             formatTimeOrZero(class.UpdatedAt),
	}
}

func positionLotToResponse(lot PositionLot) PositionLotResponse {
	return PositionLotResponse{
		ID:           lot.ID.Hex(),
		AssetID:      lot.AssetID.Hex(),
		Quantity:     lot.Quantity,
		UnitCost:     lot.UnitCost,
		CostCurrency: lot.CostCurrency,
		Fees:         lot.Fees,
		Notes:        lot.Notes,
		FxPair:       lot.FxPair,
		FxRateUsed:   lot.FxRateUsed,
		AcquiredAt:   formatTimeOrZero(lot.AcquiredAt),
		CreatedAt:    formatTimeOrZero(lot.CreatedAt),
		UpdatedAt:    formatTimeOrZero(lot.UpdatedAt),
	}
}

func cashFlowToResponse(flow CashFlow) CashFlowResponse {
	return CashFlowResponse{
		ID:         flow.ID.Hex(),
		AssetID:    flow.AssetID.Hex(),
		Type:       flow.Type,
		Amount:     flow.Amount,
		Currency:   flow.Currency,
		OccurredAt: formatTimeOrZero(flow.OccurredAt),
		Notes:      flow.Notes,
		CreatedAt:  formatTimeOrZero(flow.CreatedAt),
	}
}

func valuationToResponse(snapshot ValuationSnapshot) ValuationSnapshotResponse {
	return ValuationSnapshotResponse{
		ID:                   snapshot.ID.Hex(),
		AssetID:              snapshot.AssetID.Hex(),
		CapturedAt:           formatTimeOrZero(snapshot.CapturedAt),
		NativeCurrencyValue:  snapshot.NativeCurrencyValue,
		NativeCurrency:       snapshot.NativeCurrency,
		ValuationCurrencyMap: snapshot.ValuationCurrencyMap,
		RatesUsed:            snapshot.RatesUsed,
		Notes:                snapshot.Notes,
		Source:               snapshot.Source,
		CreatedAt:            formatTimeOrZero(snapshot.CreatedAt),
	}
}

func fxRateToResponse(rate FxRate) FxRateResponse {
	return FxRateResponse{
		ID:            rate.ID.Hex(),
		BaseCurrency:  rate.BaseCurrency,
		QuoteCurrency: rate.QuoteCurrency,
		Rate:          rate.Rate,
		EffectiveAt:   formatTimeOrZero(rate.EffectiveAt),
		Source:        rate.Source,
		CreatedAt:     formatTimeOrZero(rate.CreatedAt),
	}
}

func (h *Handler) fetchAsset(ctx context.Context, id primitive.ObjectID, userID string) (*Asset, error) {
	var asset Asset
	err := h.assetsCollection().FindOne(ctx, bson.M{
		"_id":     id,
		"user_id": userID,
	}).Decode(&asset)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, fmt.Errorf("fetch asset: %w", err)
	}
	return &asset, nil
}

func (h *Handler) getLatestValuation(ctx context.Context, assetID primitive.ObjectID, userID string) (*ValuationSnapshot, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "captured_at", Value: -1}})
	var snapshot ValuationSnapshot
	err := h.valuationsCollection().FindOne(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	}, opts).Decode(&snapshot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest valuation: %w", err)
	}
	return &snapshot, nil
}

func (h *Handler) getLatestFxRate(ctx context.Context, baseCurrency, quoteCurrency string) (*FxRate, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "effective_at", Value: -1}})
	var rate FxRate
	err := h.fxRatesCollection().FindOne(ctx, bson.M{
		"base_currency":  baseCurrency,
		"quote_currency": quoteCurrency,
	}, opts).Decode(&rate)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest fx rate: %w", err)
	}
	return &rate, nil
}

func (h *Handler) convertAmount(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, error) {
	from := strings.ToUpper(strings.TrimSpace(fromCurrency))
	to := strings.ToUpper(strings.TrimSpace(toCurrency))

	if from == "" || to == "" {
		return 0, fmt.Errorf("currency codes must be provided")
	}

	if from == to {
		return amount, nil
	}

	rate, err := h.getLatestFxRate(ctx, from, to)
	if err != nil {
		return 0, err
	}
	if rate != nil {
		return amount * rate.Rate, nil
	}

	// Try inverse rate
	inverseRate, err := h.getLatestFxRate(ctx, to, from)
	if err != nil {
		return 0, err
	}
	if inverseRate != nil && inverseRate.Rate != 0 {
		return amount / inverseRate.Rate, nil
	}

	// Try via reference currency
	reference := strings.ToUpper(strings.TrimSpace(h.config.DefaultReferenceCurrency))
	if reference == "" || reference == from || reference == to {
		return 0, fmt.Errorf("fx rate not found for %s -> %s", from, to)
	}

	toReference, err := h.convertAmount(ctx, amount, from, reference)
	if err != nil {
		return 0, err
	}
	return h.convertAmount(ctx, toReference, reference, to)
}

func validateCashFlowType(flowType CashFlowType) bool {
	switch flowType {
	case CashFlowDeposit, CashFlowWithdrawal, CashFlowDividend, CashFlowInterest:
		return true
	default:
		return false
	}
}
