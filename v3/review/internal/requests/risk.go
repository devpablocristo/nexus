package requests

import (
	requestdomain "github.com/devpablocristo/nexus/v3/review/internal/requests/usecases/domain"
)

type RiskConfig struct {
	HighActionTypes   map[string]bool
	MediumActionTypes map[string]bool
}

func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		HighActionTypes:   map[string]bool{"alert.silence": true, "runbook.execute": true},
		MediumActionTypes: map[string]bool{"incident.resolve": true},
	}
}

func TierRisk(actionType string, policyRiskOverride *string, cfg RiskConfig) requestdomain.RiskLevel {
	if policyRiskOverride != nil {
		switch *policyRiskOverride {
		case "high":
			return requestdomain.RiskHigh
		case "medium":
			return requestdomain.RiskMedium
		case "low":
			return requestdomain.RiskLow
		}
	}
	if cfg.HighActionTypes[actionType] {
		return requestdomain.RiskHigh
	}
	if cfg.MediumActionTypes[actionType] {
		return requestdomain.RiskMedium
	}
	return requestdomain.RiskLow
}

func DecideFromPolicy(effect string, risk requestdomain.RiskLevel) (requestdomain.Decision, bool) {
	switch effect {
	case "deny":
		return requestdomain.DecisionDeny, true
	case "require_approval":
		return requestdomain.DecisionRequireApproval, true
	case "allow":
		if risk == requestdomain.RiskHigh {
			return requestdomain.DecisionRequireApproval, true
		}
		return requestdomain.DecisionAllow, true
	}
	return "", false
}

func DefaultDecision(risk requestdomain.RiskLevel) requestdomain.Decision {
	if risk == requestdomain.RiskHigh {
		return requestdomain.DecisionRequireApproval
	}
	return requestdomain.DecisionAllow
}
