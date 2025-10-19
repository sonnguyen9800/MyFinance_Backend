package portfolio

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *Handler) HandleGetPortfolioSummary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	baseCurrency := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("base", h.config.DefaultReferenceCurrency)))
	if baseCurrency == "" {
		baseCurrency = strings.ToUpper(strings.TrimSpace(h.config.DefaultReferenceCurrency))
		if baseCurrency == "" {
			baseCurrency = "USD"
		}
	}

	ctx, cancel := h.newContext()
	defer cancel()

	cursor, err := h.assetsCollection().Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch assets"})
		return
	}
	defer cursor.Close(ctx)

	var assets []Asset
	if err := cursor.All(ctx, &assets); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode assets"})
		return
	}

	summaries := make([]PortfolioPositionSummary, 0, len(assets))
	var totalCurrent, totalCost float64

	for _, asset := range assets {
		summary, err := h.summarizeAsset(ctx, userID, asset, baseCurrency)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		totalCurrent += summary.CurrentValue
		totalCost += summary.CostBasis
		summaries = append(summaries, summary)
	}

	response := PortfolioSummaryResponse{
		BaseCurrency:        baseCurrency,
		TotalCurrentValue:   totalCurrent,
		TotalCostBasis:      totalCost,
		TotalUnrealizedGain: totalCurrent - totalCost,
		Positions:           summaries,
		GeneratedAt:         time.Now().UTC(),
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) summarizeAsset(ctx context.Context, userID string, asset Asset, baseCurrency string) (PortfolioPositionSummary, error) {
	var summary PortfolioPositionSummary
	summary.Asset = assetToResponse(asset)

	snapshot, err := h.getLatestValuation(ctx, asset.ID, userID)
	if err != nil {
		return summary, err
	}

	if snapshot != nil {
		value, err := h.valueForCurrency(ctx, *snapshot, baseCurrency)
		if err == nil {
			summary.CurrentValue = value
			last := snapshot.CapturedAt
			summary.LastValuationAt = &last
		}
	}

	costBasis, err := h.aggregateCostBasis(ctx, userID, asset.ID, baseCurrency)
	if err != nil {
		return summary, err
	}
	summary.CostBasis = costBasis

	gain := summary.CurrentValue - costBasis
	summary.UnrealizedGain = gain
	if costBasis != 0 {
		summary.UnrealizedGainPercent = (gain / costBasis) * 100
	}

	return summary, nil
}

func (h *Handler) valueForCurrency(ctx context.Context, snapshot ValuationSnapshot, targetCurrency string) (float64, error) {
	target := strings.ToUpper(targetCurrency)
	if target == "" {
		target = strings.ToUpper(h.config.DefaultReferenceCurrency)
	}

	if target == "" {
		target = "USD"
	}

	if snapshot.ValuationCurrencyMap != nil {
		if value, ok := snapshot.ValuationCurrencyMap[target]; ok {
			return value, nil
		}
	}

	return h.convertAmount(ctx, snapshot.NativeCurrencyValue, snapshot.NativeCurrency, target)
}

func (h *Handler) aggregateCostBasis(ctx context.Context, userID string, assetID primitive.ObjectID, baseCurrency string) (float64, error) {
	var total float64

	if err := h.sumLotsCost(ctx, userID, assetID, baseCurrency, &total); err != nil {
		return 0, err
	}

	if err := h.sumCashFlows(ctx, userID, assetID, baseCurrency, &total); err != nil {
		return 0, err
	}

	return total, nil
}

func (h *Handler) sumLotsCost(ctx context.Context, userID string, assetID primitive.ObjectID, baseCurrency string, total *float64) error {
	cursor, err := h.positionLotsCollection().Find(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var lot PositionLot
		if err := cursor.Decode(&lot); err != nil {
			return err
		}
		amount := (lot.Quantity * lot.UnitCost) + lot.Fees
		value, err := h.convertAmount(ctx, amount, lot.CostCurrency, baseCurrency)
		if err != nil {
			continue
		}
		*total += value
	}

	return cursor.Err()
}

func (h *Handler) sumCashFlows(ctx context.Context, userID string, assetID primitive.ObjectID, baseCurrency string, total *float64) error {
	cursor, err := h.cashFlowsCollection().Find(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var flow CashFlow
		if err := cursor.Decode(&flow); err != nil {
			return err
		}

		amount := flow.Amount
		if flow.Type == CashFlowWithdrawal {
			amount = -amount
		}

		value, err := h.convertAmount(ctx, amount, flow.Currency, baseCurrency)
		if err != nil {
			continue
		}
		*total += value
	}

	return cursor.Err()
}
