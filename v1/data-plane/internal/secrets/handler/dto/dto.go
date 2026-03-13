package dto

type UpsertSecretRequest struct {
	SecretType string `json:"secret_type" binding:"required"`
	KeyName    string `json:"key_name" binding:"required"`
	Value      string `json:"value" binding:"required"`
	Enabled    *bool  `json:"enabled"`
}

type SecretResponse struct {
	ID         string `json:"id"`
	SecretType string `json:"secret_type"`
	KeyName    string `json:"key_name"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
