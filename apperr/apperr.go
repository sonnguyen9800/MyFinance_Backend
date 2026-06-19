// Package apperr provides typed application errors and a uniform JSON error
// envelope: {"error": {"code": "...", "message": "..."}}.
//
// Handlers can return / respond with these instead of ad-hoc gin.H strings.
// Adopt incrementally — the existing string shape ({"error": "msg"}) still works.
package apperr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

func New(status int, code, msg string) *AppError {
	return &AppError{Status: status, Code: code, Message: msg}
}


// Constructors for the common cases.
func BadRequest(msg string) *AppError     { return New(http.StatusBadRequest, "bad_request", msg) }
func Unauthorized(msg string) *AppError   { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *AppError      { return New(http.StatusForbidden, "forbidden", msg) }
func NotFound(msg string) *AppError       { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(msg string) *AppError       { return New(http.StatusConflict, "conflict", msg) }
func Internal(msg string) *AppError       { return New(http.StatusInternalServerError, "internal", msg) }
func NotImplemented(msg string) *AppError { return New(http.StatusNotImplemented, "not_implemented", msg) }

// Respond writes the uniform error envelope for an *AppError; anything else
// becomes a generic 500.
func Respond(c *gin.Context, err error) {
	if ae, ok := err.(*AppError); ok {
		c.JSON(ae.Status, gin.H{"error": gin.H{"code": ae.Code, "message": ae.Message}})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "Internal server error"}})
}
