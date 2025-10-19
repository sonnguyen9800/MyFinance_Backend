package portfolio

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (h *Handler) HandleCreateFxRate(c *gin.Context) {
	userID := c.GetString("user_id")
	scope := strings.ToLower(strings.TrimSpace(c.DefaultQuery("scope", "global")))

	var req CreateFxRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	base := strings.ToUpper(strings.TrimSpace(req.BaseCurrency))
	quote := strings.ToUpper(strings.TrimSpace(req.QuoteCurrency))
	if base == "" || quote == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Currency codes are required"})
		return
	}

	if base == quote {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Base and quote currencies must differ"})
		return
	}

	effectiveAt, err := parseTimeOrDefault(req.EffectiveAt, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid effective_at timestamp"})
		return
	}

	now := time.Now().UTC()
	rate := FxRate{
		ID:            primitive.NewObjectID(),
		BaseCurrency:  base,
		QuoteCurrency: quote,
		Rate:          req.Rate,
		EffectiveAt:   effectiveAt,
		Source:        req.Source,
		CreatedAt:     now,
	}

	if scope == "user" {
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		rate.UserID = userID
	}

	ctx, cancel := h.newContext()
	defer cancel()

	if _, err := h.fxRatesCollection().InsertOne(ctx, rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create FX rate"})
		return
	}

	c.JSON(http.StatusCreated, fxRateToResponse(rate))
}

func (h *Handler) HandleGetFxRates(c *gin.Context) {
	userID := c.GetString("user_id")
	scope := strings.ToLower(strings.TrimSpace(c.DefaultQuery("scope", "global")))

	filter := bson.M{}
	if base := strings.TrimSpace(c.Query("base")); base != "" {
		filter["base_currency"] = strings.ToUpper(base)
	}
	if quote := strings.TrimSpace(c.Query("quote")); quote != "" {
		filter["quote_currency"] = strings.ToUpper(quote)
	}

	if scope == "user" {
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		filter["user_id"] = userID
	} else {
		filter["user_id"] = bson.M{"$exists": false}
	}

	opts := options.Find().SetSort(bson.D{{Key: "effective_at", Value: -1}}).SetLimit(200)

	ctx, cancel := h.newContext()
	defer cancel()

	cursor, err := h.fxRatesCollection().Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch FX rates"})
		return
	}
	defer cursor.Close(ctx)

	var rates []FxRate
	if err := cursor.All(ctx, &rates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode FX rates"})
		return
	}

	responses := make([]FxRateResponse, 0, len(rates))
	for _, rate := range rates {
		responses = append(responses, fxRateToResponse(rate))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) HandleDeleteFxRate(c *gin.Context) {
	userID := c.GetString("user_id")
	scope := strings.ToLower(strings.TrimSpace(c.DefaultQuery("scope", "global")))

	rateID, err := toObjectID(c.Param("rateId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid FX rate ID"})
		return
	}

	filter := bson.M{"_id": rateID}
	if scope == "user" {
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		filter["user_id"] = userID
	} else {
		filter["user_id"] = bson.M{"$exists": false}
	}

	ctx, cancel := h.newContext()
	defer cancel()

	result, err := h.fxRatesCollection().DeleteOne(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete FX rate"})
		return
	}
	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "FX rate not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
