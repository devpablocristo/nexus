package dto

type UpsertRuleRequest struct {
	Host    string `json:"host" binding:"required"`
	Enabled *bool  `json:"enabled"`
}
