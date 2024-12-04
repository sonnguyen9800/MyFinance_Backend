package tag

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct {
	mongoClient *mongo.Client
}

func NewHandler(mongoClient *mongo.Client) *Handler {
	return &Handler{
		mongoClient: mongoClient,
	}
}

// Create tag
func (h *Handler) HandleCreateTag(c *gin.Context) {
	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("tags")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if tag with same name exists
	existingFilter := bson.M{"name": req.Name}
	var existingTag Tag
	err := collection.FindOne(ctx, existingFilter).Decode(&existingTag)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag with this name already exists"})
		return
	}

	tag := Tag{
		ID:   primitive.NewObjectID().Hex(),
		Name: req.Name,
	}

	_, err = collection.InsertOne(ctx, tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create tag"})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// Get all tags
func (h *Handler) HandleGetTags(c *gin.Context) {
	collection := h.mongoClient.Database("MyFinance_Dev").Collection("tags")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch tags"})
		return
	}
	defer cursor.Close(ctx)

	var tags []Tag
	if err = cursor.All(ctx, &tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not decode tags"})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// Get single tag
func (h *Handler) HandleGetTag(c *gin.Context) {
	tagID := c.Param("id")

	collection := h.mongoClient.Database("MyFinance_Dev").Collection("tags")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var tag Tag
	err := collection.FindOne(ctx, bson.M{"_id": tagID}).Decode(&tag)

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}
