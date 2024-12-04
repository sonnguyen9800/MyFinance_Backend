package category

import (
	"context"
	"my-finance-backend/config"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	DefaultCategoryName     = "Default"
	DefaultCategoryColor    = "#000000"
	DefaultCategoryIconName = "fa-flutter"
)

type Handler struct {
	mongoClient *mongo.Client
	config      *config.Config
}

func NewHandler(mongoClient *mongo.Client, config *config.Config) *Handler {
	handler := &Handler{
		mongoClient: mongoClient,
		config:      config,
	}
	// Initialize default category if not exists
	handler.initializeDefaultCategory()
	return handler
}

func (h *Handler) initializeDefaultCategory() {
	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if default category exists
	filter := bson.M{"name": DefaultCategoryName}
	var existingCategory Category
	err := collection.FindOne(ctx, filter).Decode(&existingCategory)

	if err == mongo.ErrNoDocuments {
		// Create default category
		defaultCategory := Category{
			Name:     DefaultCategoryName,
			Color:    DefaultCategoryColor,
			IconName: DefaultCategoryIconName,
		}

		_, err := collection.InsertOne(ctx, defaultCategory)
		if err != nil {
			// Log error but don't panic as the service can still function
			println("Error creating default category:", err.Error())
		}
	}
}

// Create category
func (h *Handler) HandleCreateCategory(c *gin.Context) {
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

	// Check if category with same name exists
	existingFilter := bson.M{"name": req.Name}
	var existingCategory Category
	err := collection.FindOne(ctx, existingFilter).Decode(&existingCategory)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Category with this name already exists"})
		return
	}

	category := Category{
		Name:     req.Name,
		Color:    req.Color,
		IconName: req.IconName,
	}

	_, err = collection.InsertOne(ctx, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// Get all categories
func (h *Handler) HandleGetCategories(c *gin.Context) {
	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch categories"})
		return
	}
	defer cursor.Close(ctx)

	var categories []Category
	if err = cursor.All(ctx, &categories); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// Get single category
func (h *Handler) HandleGetCategory(c *gin.Context) {
	categoryID := c.Param("id")

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var category Category
	err := collection.FindOne(ctx, bson.M{"_id": categoryID}).Decode(&category)

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
	categoryID := c.Param("id")

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if trying to update default category
	var existingCategory Category
	err := collection.FindOne(ctx, bson.M{"_id": categoryID}).Decode(&existingCategory)
	if err == nil && existingCategory.Name == DefaultCategoryName {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify default category"})
		return
	}

	// Check if new name conflicts with default or existing category
	if req.Name != "" && req.Name != existingCategory.Name {
		if req.Name == DefaultCategoryName {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot use reserved name 'Default'"})
			return
		}

		// Check for name conflict with other categories
		var conflictCategory Category
		err := collection.FindOne(ctx, bson.M{"name": req.Name, "_id": bson.M{"$ne": categoryID}}).Decode(&conflictCategory)
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
		bson.M{"_id": categoryID},
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
	err = collection.FindOne(ctx, bson.M{"_id": categoryID}).Decode(&updatedCategory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch updated category"})
		return
	}

	c.JSON(http.StatusOK, updatedCategory)
}

// Delete category
func (h *Handler) HandleDeleteCategory(c *gin.Context) {
	categoryID := c.Param("id")

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionCategoriesName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if trying to delete default category
	var category Category
	err := collection.FindOne(ctx, bson.M{"_id": categoryID}).Decode(&category)
	if err == nil && category.Name == DefaultCategoryName {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete default category"})
		return
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": categoryID})
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
