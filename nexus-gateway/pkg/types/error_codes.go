package types

const (
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeValidation          = "VALIDATION_ERROR"
	ErrCodeSchemaInvalid       = "SCHEMA_INVALID"
	ErrCodePolicyDenied        = "POLICY_DENIED"
	ErrCodeRateLimited         = "RATE_LIMITED"
	ErrCodeUpstream5xx         = "UPSTREAM_5XX"
	ErrCodeNetworkError        = "NETWORK_ERROR"
	ErrCodeTimeout             = "TIMEOUT"
	ErrCodeResponseTooLarge    = "RESPONSE_TOO_LARGE"
	ErrCodeOutputSchemaInvalid = "OUTPUT_SCHEMA_INVALID"
	ErrCodeInvalidGETInput     = "INVALID_GET_INPUT"
	ErrCodeInternal            = "INTERNAL"
)
