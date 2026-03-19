package dto

import configdomain "github.com/devpablocristo/nexus/v3/review/internal/config/usecases/domain"

// RiskConfigResponse es la respuesta completa de configuración de riesgo
type RiskConfigResponse struct {
	Thresholds          configdomain.Thresholds          `json:"thresholds"`
	ActionTypes         configdomain.ActionTypeRisk       `json:"action_types"`
	BusinessHours       configdomain.BusinessHours        `json:"business_hours"`
	FrequencyThresholds configdomain.FrequencyThresholds  `json:"frequency_thresholds"`
	ActorThresholds     configdomain.ActorThresholds      `json:"actor_thresholds"`
	Amplifications      []configdomain.Amplification      `json:"amplifications"`
	SensitiveSystems    []string                          `json:"sensitive_systems"`
}

// UpdateRiskConfigRequest es el body para actualizar la configuración
type UpdateRiskConfigRequest = RiskConfigResponse

// ToResponse convierte domain → DTO
func ToResponse(cfg *configdomain.RiskConfig) RiskConfigResponse {
	return RiskConfigResponse{
		Thresholds:          cfg.Thresholds,
		ActionTypes:         cfg.ActionTypes,
		BusinessHours:       cfg.BusinessHours,
		FrequencyThresholds: cfg.FrequencyThresholds,
		ActorThresholds:     cfg.ActorThresholds,
		Amplifications:      cfg.Amplifications,
		SensitiveSystems:    cfg.SensitiveSystems,
	}
}

// ToDomain convierte DTO → domain
func ToDomain(r UpdateRiskConfigRequest) configdomain.RiskConfig {
	return configdomain.RiskConfig{
		Thresholds:          r.Thresholds,
		ActionTypes:         r.ActionTypes,
		BusinessHours:       r.BusinessHours,
		FrequencyThresholds: r.FrequencyThresholds,
		ActorThresholds:     r.ActorThresholds,
		Amplifications:      r.Amplifications,
		SensitiveSystems:    r.SensitiveSystems,
	}
}
