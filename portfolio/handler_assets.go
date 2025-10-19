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

func (h *Handler) HandleCreateAsset(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req CreateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	now := time.Now().UTC()
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	asset := Asset{
		ID:              primitive.NewObjectID(),
		UserID:          userID,
		AssetClassCode:  strings.ToUpper(strings.TrimSpace(req.AssetClassCode)),
		DisplayName:     strings.TrimSpace(req.DisplayName),
		DefaultCurrency: strings.ToUpper(strings.TrimSpace(req.DefaultCurrency)),
		Metadata:        req.Metadata,
		Tags:            req.Tags,
		Attributes:      req.Attributes,
		Notes:           req.Notes,
		ExpectedReturn:  req.ExpectedReturn,
		Active:          active,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	ctx, cancel := h.newContext()
	defer cancel()

	if _, err := h.assetsCollection().InsertOne(ctx, asset); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create asset"})
		return
	}

	c.JSON(http.StatusCreated, assetToResponse(asset))
}

func (h *Handler) HandleGetAssets(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	includeInactive := strings.EqualFold(strings.TrimSpace(c.DefaultQuery("include_inactive", "false")), "true")

	filter := bson.M{
		"user_id": userID,
	}
	if !includeInactive {
		filter["active"] = true
	}

	ctx, cancel := h.newContext()
	defer cancel()

	cursor, err := h.assetsCollection().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
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

	responses := make([]AssetResponse, 0, len(assets))
	for _, asset := range assets {
		responses = append(responses, assetToResponse(asset))
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) HandleGetAsset(c *gin.Context) {
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

	asset, err := h.fetchAsset(ctx, assetID, userID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch asset"})
		return
	}

	c.JSON(http.StatusOK, assetToResponse(*asset))
}

func (h *Handler) HandleUpdateAsset(c *gin.Context) {
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

	var req UpdateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	update := bson.M{}
	if req.DisplayName != nil {
		update["display_name"] = strings.TrimSpace(*req.DisplayName)
	}
	if req.DefaultCurrency != nil {
		update["default_currency"] = strings.ToUpper(strings.TrimSpace(*req.DefaultCurrency))
	}
	if req.Metadata != nil {
		update["metadata"] = req.Metadata
	}
	if req.Tags != nil {
		update["tags"] = req.Tags
	}
	if req.Attributes != nil {
		update["attributes"] = req.Attributes
	}
	if req.Notes != nil {
		update["notes"] = strings.TrimSpace(*req.Notes)
	}
	if req.ExpectedReturn != nil {
		update["expected_return"] = req.ExpectedReturn
	}
	if req.Active != nil {
		update["active"] = *req.Active
	}
	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to update"})
		return
	}

	update["updated_at"] = time.Now().UTC()

	ctx, cancel := h.newContext()
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated Asset
	err = h.assetsCollection().FindOneAndUpdate(
		ctx,
		bson.M{"_id": assetID, "user_id": userID},
		bson.M{"$set": update},
		opts,
	).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update asset"})
		return
	}

	c.JSON(http.StatusOK, assetToResponse(updated))
}

func (h *Handler) HandleDeleteAsset(c *gin.Context) {
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

	result, err := h.assetsCollection().DeleteOne(ctx, bson.M{"_id": assetID, "user_id": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete asset"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	// Cascade delete related records
	_, _ = h.positionLotsCollection().DeleteMany(ctx, bson.M{"asset_id": assetID, "user_id": userID})
	_, _ = h.cashFlowsCollection().DeleteMany(ctx, bson.M{"asset_id": assetID, "user_id": userID})
	_, _ = h.valuationsCollection().DeleteMany(ctx, bson.M{"asset_id": assetID, "user_id": userID})

	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleCreateAssetClass(c *gin.Context) {
	var req CreateAssetClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Asset class code is required"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Asset class name is required"})
		return
	}

	now := time.Now().UTC()
	class := AssetClass{
		ID:                    primitive.NewObjectID(),
		Code:                  code,
		Name:                  name,
		DefaultValuationModel: strings.TrimSpace(req.DefaultValuationModel),
		MetadataSchema:        req.MetadataSchema,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	ctx, cancel := h.newContext()
	defer cancel()

	err := h.assetClassesCollection().FindOne(ctx, bson.M{"code": code}).Err()
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Asset class with this code already exists"})
		return
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not verify asset class uniqueness"})
		return
	}

	if _, err := h.assetClassesCollection().InsertOne(ctx, class); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create asset class"})
		return
	}

	c.JSON(http.StatusCreated, assetClassToResponse(class))
}

func (h *Handler) HandleGetAssetClasses(c *gin.Context) {
	ctx, cancel := h.newContext()
	defer cancel()

	cursor, err := h.assetClassesCollection().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch asset classes"})
		return
	}
	defer cursor.Close(ctx)

	var classes []AssetClass
	if err := cursor.All(ctx, &classes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode asset classes"})
		return
	}

	responses := make([]AssetClassResponse, 0, len(classes))
	for _, class := range classes {
		responses = append(responses, assetClassToResponse(class))
	}

	c.JSON(http.StatusOK, responses)
}
