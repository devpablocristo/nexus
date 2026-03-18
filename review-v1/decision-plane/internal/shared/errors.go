package shared

import (
	"log/slog"
	"net/http"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
)

// ErrorResponse es la estructura estándar de error HTTP para todos los handlers.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError escribe un error estructurado al cliente.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}

// WriteInternalError loguea el error real y retorna un mensaje genérico al cliente.
// NUNCA expone err.Error() al cliente.
func WriteInternalError(w http.ResponseWriter, err error, context string) {
	slog.Error(context, "error", err)
	WriteError(w, http.StatusInternalServerError, "INTERNAL", context)
}

// Constantes de límites
const (
	DefaultListLimit = 50
	MaxListLimit     = 1000
	MaxExpressionLen = 5000
	MaxIdempotencyKeyLen = 256
)
