package dto

type UpsertRuleRequest struct {
	Host    string `json:"host"`
	Enabled *bool  `json:"enabled"`
}
