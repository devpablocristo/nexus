package dto

type UpsertSecretRequest struct {
	SecretType string `json:"secret_type"`
	KeyName    string `json:"key_name"`
	Value      string `json:"value"`
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
