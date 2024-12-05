package category

import (
	"context"
	"my-finance-backend/config"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	DefaultCategoryName     = "Default"
	DefaultCategoryColor    = "#000000"
	DefaultCategoryIconName = "fa-flutter"
)

type Handler struct {
	mongoClient *mongo.Client
	jwtSecret   []byte
	config      *config.Config
}

func NewHandler(mongoClient *mongo.Client, config *config.Config, jwtSecret []byte) *Handler {
	handler := &Handler{
		mongoClient: mongoClient,
		config:      config,
		jwtSecret:   jwtSecret,
	}
	return handler
}

// initializeDefaultCategory creates a default category for a specific user if it doesn't exist
func (h *Handler) initializeDefaultCategory(userID string) {
	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	// Check if default category exists for this user
	filter := bson.M{
		"name":    DefaultCategoryName,
		"user_id": userID,
	}
	var existingCategory Category
	err := collection.FindOne(ctx, filter).Decode(&existingCategory)

	if err == mongo.ErrNoDocuments {
		// Create default category for this user
		defaultCategory := Category{
			//ID:       primitive.NewObjectID().Hex(),
			UserID:   userID,
			Name:     DefaultCategoryName,
			Color:    DefaultCategoryColor,
			IconName: DefaultCategoryIconName,
		}

		_, err := collection.InsertOne(ctx, defaultCategory)
		if err != nil {
			println("Error creating default category:", err.Error())
		}
	}
}

// Create category
func (h *Handler) HandleCreateCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if name is default category
	if req.Name == DefaultCategoryName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create category with reserved name 'Default'"})
		return
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize default category if it doesn't exist
	h.initializeDefaultCategory(userID)

	// Check if category with same name exists for this user
	existingFilter := bson.M{
		"name":    req.Name,
		"user_id": userID,
	}
	var existingCategory Category
	err := collection.FindOne(ctx, existingFilter).Decode(&existingCategory)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Category with this name already exists"})
		return
	}

	category := Category{
		UserID:   userID,
		Name:     req.Name,
		Color:    req.Color,
		IconName: req.IconName,
	}

	result, err := collection.InsertOne(ctx, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create category"})
		return
	}
	category.ID = result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusCreated, category)
}

// Get all categories for a user
func (h *Handler) HandleGetCategories(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	// Initialize default category if it doesn't exist
	h.initializeDefaultCategory(userID)

	// Find all categories for this user
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch categories"})
		return
	}
	defer cursor.Close(ctx)

	var categories []Category = make([]Category, 0)
	if err = cursor.All(ctx, &categories); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// Get single category
func (h *Handler) HandleGetCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	categoryID := c.Param("id")

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var category Category
	err := collection.FindOne(ctx, bson.M{
		"_id":     categoryID,
		"user_id": userID,
	}).Decode(&category)

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch category"})
		return
	}

	c.JSON(http.StatusOK, category)
}

// Update category
func (h *Handler) HandleUpdateCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	categoryID := c.Param("id")

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if trying to update default category
	var existingCategory Category
	err := collection.FindOne(ctx, bson.M{
		"_id":     categoryID,
		"user_id": userID,
	}).Decode(&existingCategory)

	if (err == context.DeadlineExceeded) || (err == context.Canceled) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch category. Too long to respond."})
		return
	}

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	if existingCategory.Name == DefaultCategoryName {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify default category"})
		return
	}

	// Check if new name conflicts with default or existing category
	if req.Name != "" && req.Name != existingCategory.Name {
		if req.Name == DefaultCategoryName {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot use reserved name 'Default'"})
			return
		}

		// Check for name conflict with other categories for this user
		var conflictCategory Category
		err := collection.FindOne(ctx, bson.M{
			"name":    req.Name,
			"user_id": userID,
			"_id":     bson.M{"$ne": categoryID},
		}).Decode(&conflictCategory)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Category with this name already exists"})
			return
		}
	}

	// Build update document
	update := bson.M{}
	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Color != "" {
		update["color"] = req.Color
	}
	if req.IconName != "" {
		update["icon_name"] = req.IconName
	}

	result, err := collection.UpdateOne(
		ctx,
		bson.M{
			"_id":     categoryID,
			"user_id": userID,
		},
		bson.M{"$set": update},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update category"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Get updated category
	var updatedCategory Category
	err = collection.FindOne(ctx, bson.M{
		"_id":     categoryID,
		"user_id": userID,
	}).Decode(&updatedCategory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch updated category"})
		return
	}

	c.JSON(http.StatusOK, updatedCategory)
}

// Delete category
func (h *Handler) HandleDeleteCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	categoryID := c.Param("id")

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("categories")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if trying to delete default category
	var category Category
	err := collection.FindOne(ctx, bson.M{
		"_id":     categoryID,
		"user_id": userID,
	}).Decode(&category)

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	if category.Name == DefaultCategoryName {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete default category"})
		return
	}

	result, err := collection.DeleteOne(ctx, bson.M{
		"_id":     categoryID,
		"user_id": userID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete category"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}
