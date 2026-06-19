package category

import (
	"context"
	"my-finance-backend/apperr"
	"my-finance-backend/config"
	"net/http"
	"strings"
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
	jwtSecret []byte
	config    *config.Config
	repo      *Repository
}

func NewHandler(mongoClient *mongo.Client, config *config.Config, jwtSecret []byte) *Handler {
	return &Handler{
		config:    config,
		jwtSecret: jwtSecret,
		repo:      NewRepository(mongoClient.Database(config.DatabaseName), config.CollectionCategoriesName, config.CollectionExpensesName),
	}
}

func (h *Handler) HandleCreateCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == DefaultCategoryName {
		apperr.Respond(c, apperr.BadRequest("Cannot create category with reserved name 'Default'"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = h.repo.EnsureDefault(ctx, userID)

	if _, err := h.repo.FindByName(ctx, userID, req.Name); err == nil {
		apperr.Respond(c, apperr.Conflict("Category with this name already exists"))
		return
	} else if err != mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.Internal("Database error"))
		return
	}

	cat := &Category{UserID: userID, Name: req.Name, Color: req.Color, IconName: req.IconName}
	if err := h.repo.Create(ctx, cat); err != nil {
		apperr.Respond(c, apperr.Internal("Could not create category"))
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *Handler) HandleGetCategories(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = h.repo.EnsureDefault(ctx, userID)
	cats, err := h.repo.ListByUser(ctx, userID)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch categories"))
		return
	}
	c.JSON(http.StatusOK, cats)
}

func (h *Handler) HandleGetCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid category ID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cat, err := h.repo.GetByID(ctx, userID, oid) // ownership enforced
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Category not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch category"))
		return
	}
	c.JSON(http.StatusOK, cat)
}

func (h *Handler) HandleUpdateCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid category ID"))
		return
	}
	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	existing, err := h.repo.GetByID(ctx, userID, oid)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Category not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch category"))
		return
	}

	if existing.Name == DefaultCategoryName && req.Name != "" && req.Name != existing.Name {
		apperr.Respond(c, apperr.Forbidden("Cannot modify default category's name"))
		return
	}
	if req.Name != "" && req.Name != existing.Name {
		if req.Name == DefaultCategoryName {
			apperr.Respond(c, apperr.BadRequest("Cannot use reserved name 'Default'"))
			return
		}
		conflict, err := h.repo.NameConflict(ctx, userID, req.Name, oid)
		if err != nil {
			apperr.Respond(c, apperr.Internal("Database error"))
			return
		}
		if conflict {
			apperr.Respond(c, apperr.Conflict("Category with this name already exists"))
			return
		}
	}

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

	if err := h.repo.Update(ctx, userID, oid, update); err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Category not found"))
		return
	} else if err != nil {
		apperr.Respond(c, apperr.Internal("Could not update category"))
		return
	}

	updated, err := h.repo.GetByID(ctx, userID, oid)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch updated category"))
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) HandleDeleteCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		apperr.Respond(c, apperr.Unauthorized("User ID not found"))
		return
	}
	oid, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid category ID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	existing, err := h.repo.GetByID(ctx, userID, oid)
	if err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Category not found"))
		return
	}
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not fetch category"))
		return
	}
	if existing.Name == DefaultCategoryName {
		apperr.Respond(c, apperr.Forbidden("Cannot delete default category"))
		return
	}

	if err := h.repo.Delete(ctx, userID, oid); err == mongo.ErrNoDocuments {
		apperr.Respond(c, apperr.NotFound("Category not found"))
		return
	} else if err != nil {
		apperr.Respond(c, apperr.Internal("Could not delete category"))
		return
	}

	// Cascade: reassign this category's expenses to the user's Default
	_ = h.repo.EnsureDefault(ctx, userID)
	if def, e := h.repo.FindByName(ctx, userID, DefaultCategoryName); e == nil {
		h.repo.ReassignExpenses(ctx, userID, oid.Hex(), def.ID)
	} else {
		h.repo.ReassignExpenses(ctx, userID, oid.Hex(), "")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}
