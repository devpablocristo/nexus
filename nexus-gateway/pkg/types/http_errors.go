package types

import "net/http"

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	RequestID string   `json:"request_id"`
	Error     APIError `json:"error"`
}

type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e HTTPError) Error() string { return e.Code + ": " + e.Message }

func NewHTTPError(status int, code, message string) HTTPError {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return HTTPError{Status: status, Code: code, Message: message}
}
