package errors

import (
	stderrs "errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"nexus-core/pkg/types"
)

func Normalize(err error) (int, types.APIError) {
	var he types.HTTPError
	if stderrs.As(err, &he) {
		status := he.Status
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return status, types.APIError{Code: he.Code, Message: he.Message}
	}
	return http.StatusInternalServerError, types.APIError{Code: types.ErrCodeInternal, Message: "internal error"}
}

func Write(c *gin.Context, status int, code, message string) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	c.AbortWithStatusJSON(status, types.ErrorResponse{
		RequestID: requestIDFromContext(c),
		Error:     types.APIError{Code: code, Message: message},
	})
}

func WriteFrom(c *gin.Context, err error) {
	status, apiErr := Normalize(err)
	Write(c, status, apiErr.Code, apiErr.Message)
}

func BadRequest(c *gin.Context, message string) {
	Write(c, http.StatusBadRequest, types.ErrCodeValidation, message)
}

func Unauthorized(c *gin.Context, message string) {
	Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, message)
}

func requestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyRequestID)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
