package portfolio

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (h *Handler) HandleCreatePositionLot(c *gin.Context) {
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

	var req CreatePositionLotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	acquiredAt, err := parseTimeOrDefault(req.AcquiredAt, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid acquired_at timestamp"})
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

	now := time.Now().UTC()
	lot := PositionLot{
		ID:           primitive.NewObjectID(),
		AssetID:      assetID,
		UserID:       userID,
		AcquiredAt:   acquiredAt,
		Quantity:     req.Quantity,
		UnitCost:     req.UnitCost,
		CostCurrency: strings.ToUpper(strings.TrimSpace(req.CostCurrency)),
		Fees:         req.Fees,
		Notes:        req.Notes,
		FxPair:       strings.ToUpper(strings.TrimSpace(req.FxPair)),
		FxRateUsed:   req.FxRateUsed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := h.positionLotsCollection().InsertOne(ctx, lot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create position lot"})
		return
	}

	c.JSON(http.StatusCreated, positionLotToResponse(lot))
}

func (h *Handler) HandleGetPositionLots(c *gin.Context) {
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

	cursor, err := h.positionLotsCollection().Find(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	}, options.Find().SetSort(bson.D{{Key: "acquired_at", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch position lots"})
		return
	}
	defer cursor.Close(ctx)

	var lots []PositionLot
	if err := cursor.All(ctx, &lots); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode position lots"})
		return
	}

	responses := make([]PositionLotResponse, 0, len(lots))
	for _, lot := range lots {
		responses = append(responses, positionLotToResponse(lot))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) HandleUpdatePositionLot(c *gin.Context) {
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

	lotID, err := toObjectID(c.Param("lotId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lot ID"})
		return
	}

	var req UpdatePositionLotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	update := bson.M{}
	if req.AcquiredAt != nil {
		acquiredAt, err := parseTimeOrDefault(*req.AcquiredAt, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid acquired_at timestamp"})
			return
		}
		update["acquired_at"] = acquiredAt
	}
	if req.Quantity != nil {
		update["quantity"] = *req.Quantity
	}
	if req.UnitCost != nil {
		update["unit_cost"] = *req.UnitCost
	}
	if req.CostCurrency != nil {
		update["cost_currency"] = strings.ToUpper(strings.TrimSpace(*req.CostCurrency))
	}
	if req.Fees != nil {
		update["fees"] = *req.Fees
	}
	if req.Notes != nil {
		update["notes"] = *req.Notes
	}
	if req.FxPair != nil {
		update["fx_pair"] = strings.ToUpper(strings.TrimSpace(*req.FxPair))
	}
	if req.FxRateUsed != nil {
		update["fx_rate_used"] = req.FxRateUsed
	}

	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to update"})
		return
	}

	update["updated_at"] = time.Now().UTC()

	ctx, cancel := h.newContext()
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated PositionLot
	err = h.positionLotsCollection().FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":      lotID,
			"asset_id": assetID,
			"user_id":  userID,
		},
		bson.M{"$set": update},
		opts,
	).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Position lot not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update position lot"})
		return
	}

	c.JSON(http.StatusOK, positionLotToResponse(updated))
}

func (h *Handler) HandleDeletePositionLot(c *gin.Context) {
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

	lotID, err := toObjectID(c.Param("lotId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lot ID"})
		return
	}

	ctx, cancel := h.newContext()
	defer cancel()

	result, err := h.positionLotsCollection().DeleteOne(ctx, bson.M{
		"_id":      lotID,
		"asset_id": assetID,
		"user_id":  userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete position lot"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position lot not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
