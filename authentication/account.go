package authentication

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type UpdateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// generateToken builds a signed JWT valid for 24 hours.
func (h *Handler) generateToken(userID, email, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	return token.SignedString(h.jwtSecret)
}

// HandleRefreshToken issues a fresh token for an already-authenticated user.
func (h *Handler) HandleRefreshToken(c *gin.Context) {
	userID := c.GetString("user_id")
	email := c.GetString("email")
	role := c.GetString("role")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	tokenString, err := h.generateToken(userID, email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// HandleUpdateUser updates the current user's name and/or email.
func (h *Handler) HandleUpdateUser(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionUserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{}
	if strings.TrimSpace(req.Name) != "" {
		update["name"] = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Email) != "" {
		newEmail := strings.TrimSpace(req.Email)
		// Ensure the new email is not used by another account
		var other User
		err := collection.FindOne(ctx, bson.M{"email": newEmail, "_id": bson.M{"$ne": userID}}).Decode(&other)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
			return
		} else if err != mongo.ErrNoDocuments {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		update["email"] = newEmail
	}

	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to update"})
		return
	}

	_, err := collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$set": update})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update user"})
		return
	}

	var user User
	if err := collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch user"})
		return
	}

	c.JSON(http.StatusOK, UserResponse{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role})
}

// HandleChangePassword verifies the old password, then sets a new one.
func (h *Handler) HandleChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password must be at least 6 characters"})
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionUserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	if err := collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not hash password"})
		return
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"password_hash": string(newHash)}}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// HandleDeleteAccount removes the user and all data they own (legal requirement).
func (h *Handler) HandleDeleteAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	db := h.mongoClient.Database(h.config.DatabaseName)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Delete all collections that store per-user data
	ownedCollections := []string{
		h.config.CollectionExpensesName,
		h.config.CollectionCategoriesName,
		h.config.CollectionTagsName,
		h.config.CollectionAssetsName,
		h.config.CollectionAssetClassesName,
		h.config.CollectionPositionLotsName,
		h.config.CollectionCashFlowsName,
		h.config.CollectionValuationsName,
		h.config.CollectionPricePointsName,
		h.config.CollectionFxRatesName,
	}
	for _, name := range ownedCollections {
		if name == "" {
			continue
		}
		_, _ = db.Collection(name).DeleteMany(ctx, bson.M{"user_id": userID})
	}

	// Finally delete the user record
	if _, err := db.Collection(h.config.CollectionUserName).DeleteOne(ctx, bson.M{"_id": userID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Account and all associated data deleted"})
}
