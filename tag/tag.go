package tag

import (
	"context"
	"my-finance-backend/apperr"
	"my-finance-backend/config"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct {
	config *config.Config
	repo   *Repository
}

func NewHandler(mongoClient *mongo.Client, config *config.Config) *Handler {
	return &Handler{
		config: config,
		repo:   NewRepository(mongoClient.Database(config.DatabaseName), config.CollectionTagsName, config.CollectionExpensesName),
	}
}

func (h *Handler) HandleCreateTag(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := h.repo.NameExists(ctx, userID, req.Name, nil)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Database error"))
		return
	}
	if exists {
		apperr.Respond(c, apperr.Conflict("Tag with this name already exists"))
		return
	}

	tag := &Tag{UserID: userID, Name: req.Name}
	if err := h.repo.Create(ctx, tag); err != nil {
		apperr.Respond(c, apperr.Internal("Could not create tag"))
		return
	}
	c.JSON(http.StatusCreated, tag)
}

func (h *Handler) HandleGetTags(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tags, err := h.repo.ListByUser(ctx, userID)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch tags"))
		return
	}
	c.JSON(http.StatusOK, tags)
}

func (h *Handler) HandleGetTag(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid tag ID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tag, err := h.repo.GetByID(ctx, userID, oid)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Tag not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch tag"))
		return
	}
	c.JSON(http.StatusOK, tag)
}

func (h *Handler) HandleUpdateTag(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid tag ID"))
		return
	}
	var req UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conflict, err := h.repo.NameExists(ctx, userID, req.Name, &oid)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Database error"))
		return
	}
	if conflict {
		apperr.Respond(c, apperr.Conflict("Tag with this name already exists"))
		return
	}

	if err := h.repo.UpdateName(ctx, userID, oid, req.Name); err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Tag not found"))
		return
	} else if err != nil {
		apperr.Respond(c, apperr.Internal("Could not update tag"))
		return
	}
	c.JSON(http.StatusOK, Tag{ID: oid.Hex(), UserID: userID, Name: req.Name})
}

func (h *Handler) HandleDeleteTag(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid tag ID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.repo.Delete(ctx, userID, oid); err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Tag not found"))
		return
	} else if err != nil {
		apperr.Respond(c, apperr.Internal("Could not delete tag"))
		return
	}
	h.repo.PullFromExpenses(ctx, userID, oid.Hex())
	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted successfully"})
}
