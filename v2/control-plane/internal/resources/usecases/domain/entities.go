package domain

import "time"

// ResourceType identifies the protected surface managed by the control-plane.
type ResourceType string

const (
	ResourceTypeWallet   ResourceType = "wallet"
	ResourceTypeTreasury ResourceType = "treasury"
	ResourceTypeVault    ResourceType = "vault"
)

// Criticality captures how sensitive a protected resource is.
type Criticality string

const (
	CriticalityLow      Criticality = "low"
	CriticalityMedium   Criticality = "medium"
	CriticalityHigh     Criticality = "high"
	CriticalityCritical Criticality = "critical"
)

// ProtectedResource is the source of truth for protected resources in Nexus.
type ProtectedResource struct {
	ID          string
	Type        ResourceType
	Name        string
	Environment string
	Chain       string
	Labels      map[string]string
	Criticality Criticality
	IsCanary    bool
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
