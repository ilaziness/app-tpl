package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	errcode "github.com/ilaziness/app-tpl/internal/errcode"
	"github.com/ilaziness/app-tpl/internal/response"
	"github.com/ilaziness/app-tpl/internal/validator"
)

// BindAndValidate binds and validates request parameters via gin ShouldBind.
// On failure it writes an error response and returns false.
func BindAndValidate(c *gin.Context, obj any) bool {
	if err := c.ShouldBind(obj); err != nil {
		response.Error(c, errcode.Wrap(errcode.ParamError, validator.TranslateError(err)))
		return false
	}
	return true
}

// BindUploadAndValidate binds multipart forms; maps http.MaxBytesError to a param error.
func BindUploadAndValidate(c *gin.Context, obj any) bool {
	if err := c.ShouldBind(obj); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.Error(c, errcode.WithMessage(errcode.ParamError, "upload too large"))
			return false
		}
		response.Error(c, errcode.Wrap(errcode.ParamError, validator.TranslateError(err)))
		return false
	}
	return true
}

// BindJSON binds a JSON body with binding tags.
func BindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		response.Error(c, errcode.Wrap(errcode.ParamError, validator.TranslateError(err)))
		return false
	}
	return true
}

// BindQuery binds query parameters with binding tags.
func BindQuery(c *gin.Context, obj any) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		response.Error(c, errcode.Wrap(errcode.ParamError, validator.TranslateError(err)))
		return false
	}
	return true
}

// BindURI binds URI parameters with binding tags.
func BindURI(c *gin.Context, obj any) bool {
	if err := c.ShouldBindUri(obj); err != nil {
		response.Error(c, errcode.Wrap(errcode.ParamError, validator.TranslateError(err)))
		return false
	}
	return true
}

// HandleServiceError maps service-layer errors to HTTP responses.
func HandleServiceError(c *gin.Context, err error) {
	if _, ok := errcode.As(err); ok {
		response.Error(c, err)
		return
	}
	response.Error(c, errcode.Wrap(errcode.InternalError, err))
}
