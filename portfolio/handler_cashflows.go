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

func (h *Handler) HandleCreateCashFlow(c *gin.Context) {
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

	var req CreateCashFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	flowType := CashFlowType(strings.ToUpper(string(req.Type)))
	if !validateCashFlowType(flowType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cash flow type"})
		return
	}

	occurredAt, err := parseTimeOrDefault(req.OccurredAt, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid occurred_at timestamp"})
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
	flow := CashFlow{
		ID:         primitive.NewObjectID(),
		AssetID:    assetID,
		UserID:     userID,
		Type:       flowType,
		Amount:     req.Amount,
		Currency:   strings.ToUpper(strings.TrimSpace(req.Currency)),
		OccurredAt: occurredAt,
		Notes:      req.Notes,
		CreatedAt:  now,
	}

	if _, err := h.cashFlowsCollection().InsertOne(ctx, flow); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cash flow"})
		return
	}

	c.JSON(http.StatusCreated, cashFlowToResponse(flow))
}

func (h *Handler) HandleGetCashFlows(c *gin.Context) {
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

	cursor, err := h.cashFlowsCollection().Find(ctx, bson.M{
		"asset_id": assetID,
		"user_id":  userID,
	}, options.Find().SetSort(bson.D{{Key: "occurred_at", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch cash flows"})
		return
	}
	defer cursor.Close(ctx)

	var flows []CashFlow
	if err := cursor.All(ctx, &flows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode cash flows"})
		return
	}

	responses := make([]CashFlowResponse, 0, len(flows))
	for _, flow := range flows {
		responses = append(responses, cashFlowToResponse(flow))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) HandleDeleteCashFlow(c *gin.Context) {
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

	flowID, err := toObjectID(c.Param("flowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cash flow ID"})
		return
	}

	ctx, cancel := h.newContext()
	defer cancel()

	result, err := h.cashFlowsCollection().DeleteOne(ctx, bson.M{
		"_id":      flowID,
		"asset_id": assetID,
		"user_id":  userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete cash flow"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cash flow not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
