package portfolio

import (
	"errors"
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

func (h *Handler) HandleCreateValuationSnapshot(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	assetID, err := toObjectID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	var req CreateValuationSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	capturedAt, err := parseTimeOrDefault(req.CapturedAt, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid captured_at timestamp"})
		return
	}

	ctx, cancel := h.newContext()
	defer cancel()

	if _, err := h.fetchAsset(ctx, assetID, userID); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not verify asset ownership"})
		return
	}

	var valuationMap map[string]float64
	if len(req.ValuationCurrencyMap) > 0 {
		valuationMap = make(map[string]float64, len(req.ValuationCurrencyMap))
		for k, v := range req.ValuationCurrencyMap {
			key := strings.ToUpper(strings.TrimSpace(k))
			if key == "" {
				continue
			}
			valuationMap[key] = v
		}
	}

	now := time.Now().UTC()
	snapshot := ValuationSnapshot{
		ID:                   primitive.NewObjectID(),
		AssetID:              assetID,
		UserID:               userID,
		CapturedAt:           capturedAt,
		NativeCurrencyValue:  req.NativeCurrencyValue,
		NativeCurrency:       strings.ToUpper(strings.TrimSpace(req.NativeCurrency)),
		ValuationCurrencyMap: valuationMap,
		RatesUsed:            req.RatesUsed,
		Notes:                req.Notes,
		Source:               req.Source,
		CreatedAt:            now,
	}

	if _, err := h.valuationsCollection().InsertOne(ctx, snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create valuation snapshot"})
		return
	}

	c.JSON(http.StatusCreated, valuationToResponse(snapshot))
}

func (h *Handler) HandleGetValuationSnapshots(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	assetID, err := toObjectID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	limit := int64(50)
	if l := c.Query("limit"); l != "" {
		if parsed, parseErr := strconv.ParseInt(l, 10, 64); parseErr == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	ctx, cancel := h.newContext()
	defer cancel()

	if _, err := h.fetchAsset(ctx, assetID, userID); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not verify asset ownership"})
		return
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "captured_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := h.valuationsCollection().Find(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	}, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch valuation snapshots"})
		return
	}
	defer cursor.Close(ctx)

	var snapshots []ValuationSnapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode valuation snapshots"})
		return
	}

	responses := make([]ValuationSnapshotResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		responses = append(responses, valuationToResponse(snapshot))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) HandleDeleteValuationSnapshot(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	assetID, err := toObjectID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	snapshotID, err := toObjectID(c.Param("valuationId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid valuation ID"})
		return
	}

	ctx, cancel := h.newContext()
	defer cancel()

	result, err := h.valuationsCollection().DeleteOne(ctx, bson.M{
		"_id":      snapshotID,
		"asset_id": assetID,
		"user_id":  userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete valuation snapshot"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Valuation snapshot not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
