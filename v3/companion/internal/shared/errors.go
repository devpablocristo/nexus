package shared

import (
	"log/slog"
	"net/http"

	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
)

// ErrorResponse es la estructura estándar de error HTTP.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError escribe un error estructurado al cliente.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}

// WriteInternalError loguea el error real y retorna mensaje genérico al cliente.
func WriteInternalError(w http.ResponseWriter, err error, context string) {
	slog.Error(context, "error", err)
	WriteError(w, http.StatusInternalServerError, "INTERNAL", context)
}

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)
