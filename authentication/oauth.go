package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"my-finance-backend/apperr"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type googleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// googleTokenInfo is the subset of Google's tokeninfo response we use.
type googleTokenInfo struct {
	Aud   string `json:"aud"`
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// HandleGoogleAuth verifies a Google ID token, upserts the user, and issues a
// MyFinance JWT. Opt-in: disabled unless GOOGLE_CLIENT_ID is configured.
//
// Verification uses Google's tokeninfo endpoint (dependency-free, fine for the
// low volume of a self-hosted family app). For high volume, switch to local
// verification via google.golang.org/api/idtoken.
//
// This handler is the reference adoption of the apperr uniform error envelope.
func (h *Handler) HandleGoogleAuth(c *gin.Context) {
	if h.config.GoogleClientID == "" {
		apperr.Respond(c, apperr.NotImplemented("Google sign-in is not configured on this server"))
		return
	}

	var req googleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperr.Respond(c, apperr.BadRequest("Invalid request body"))
		return
	}

	info, err := verifyGoogleIDToken(req.IDToken)
	if err != nil {
		apperr.Respond(c, apperr.Unauthorized("Invalid Google token"))
		return
	}
	if info.Aud != h.config.GoogleClientID {
		apperr.Respond(c, apperr.Unauthorized("Token audience mismatch"))
		return
	}
	if info.Iss != "accounts.google.com" && info.Iss != "https://accounts.google.com" {
		apperr.Respond(c, apperr.Unauthorized("Invalid token issuer"))
		return
	}
	if info.Email == "" {
		apperr.Respond(c, apperr.Unauthorized("Google account has no email"))
		return
	}

	collection := h.mongoClient.Database(h.config.DatabaseName).Collection(h.config.CollectionUserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err = collection.FindOne(ctx, bson.M{"email": info.Email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		user = User{
			ID:         primitive.NewObjectID().Hex(),
			Name:       info.Name,
			Email:      info.Email,
			Role:       "user",
			Provider:   "google",
			ProviderID: info.Sub,
		}
		if _, e := collection.InsertOne(ctx, user); e != nil {
			apperr.Respond(c, apperr.Internal("Could not create user"))
			return
		}
	} else if err != nil {
		apperr.Respond(c, apperr.Internal("Database error"))
		return
	} else if user.Provider == "" {
		// Link Google to an existing email/password account
		_, _ = collection.UpdateOne(ctx, bson.M{"_id": user.ID},
			bson.M{"$set": bson.M{"provider": "google", "provider_id": info.Sub}})
	}

	token, err := h.generateToken(user.ID, user.Email, user.Role)
	if err != nil {
		apperr.Respond(c, apperr.Internal("Could not generate token"))
		return
	}

	resp := LoginResponse{Token: token}
	resp.User.ID = user.ID
	resp.User.Name = user.Name
	resp.User.Email = user.Email
	resp.User.Role = user.Role
	c.JSON(http.StatusOK, resp)
}

func verifyGoogleIDToken(idToken string) (*googleTokenInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("token verification failed")
	}
	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
