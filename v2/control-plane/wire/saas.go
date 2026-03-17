package wire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	saasadmin "github.com/devpablocristo/saas-core/admin"
	saasadmindomain "github.com/devpablocristo/saas-core/admin/usecases/domain"
	saasbilling "github.com/devpablocristo/saas-core/billing"
	billingdomain "github.com/devpablocristo/saas-core/billing/usecases/domain"
	saasclerk "github.com/devpablocristo/saas-core/clerkwebhook"
	saasidentity "github.com/devpablocristo/saas-core/identity"
	identityoidc "github.com/devpablocristo/saas-core/identity/executor/oidc"
	saasmigrations "github.com/devpablocristo/saas-core/migrations"
	saasorg "github.com/devpablocristo/saas-core/org"
	saasmiddleware "github.com/devpablocristo/saas-core/shared/middleware"
	saasmetering "github.com/devpablocristo/saas-core/usagemetering"
	saasusers "github.com/devpablocristo/saas-core/users"
	userdomain "github.com/devpablocristo/saas-core/users/usecases/domain"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SaaSConfig holds configuration for saas-core modules.
type SaaSConfig struct {
	DatabaseURL string

	// Stripe
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripePriceStarter    string
	StripePriceGrowth     string
	StripePriceEnterprise string
	TowerBaseURL          string

	// Clerk
	ClerkWebhookSecret string

	// JWT / OIDC auth
	JWTIssuer      string
	JWTAudience    string
	JWTOrgClaim    string
	JWTRoleClaim   string
	JWTScopesClaim string
	JWTActorClaim  string
}

// SaaSServices holds initialized saas-core services.
type SaaSServices struct {
	OrgHandler     *saasorg.Handler
	UsersHandler   *saasusers.Handler
	ClerkHandler   *saasclerk.Handler
	BillingHandler *saasbilling.Handler
	AdminHandler   *saasadmin.Handler
	AuthMiddleware func(http.Handler) http.Handler
	MeteringMW     func(http.Handler) http.Handler
	Cleanup        func()
}

// SetupSaaS initializes all saas-core modules.
func SetupSaaS(cfg SaaSConfig) (*SaaSServices, error) {
	if cfg.DatabaseURL == "" {
		return nil, nil
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := saasmigrations.MigrateUp(context.Background(), sqlDB, "saas-core"); err != nil {
		return nil, err
	}

	logger := slog.Default()

	// Org
	orgRepo := saasorg.NewRepository(db)
	orgHandler := saasorg.NewHandler(orgRepo)
	orgUC := saasorg.NewUsecases(orgRepo)

	// Users
	usersRepo := saasusers.NewRepository(db)
	usersUC := saasusers.NewUsecases(usersRepo)
	usersHandler := saasusers.NewHandler(usersUC)

	// Clerk webhook — adapt users.Usecases to clerkwebhook.UserSyncer
	clerkHandler := saasclerk.NewHandler(saasclerk.Config{
		ClerkWebhookSecret: cfg.ClerkWebhookSecret,
		TowerBaseURL:       cfg.TowerBaseURL,
	}, &userSyncerAdapter{uc: usersUC}, nil, logger)

	// Admin
	adminRepo := saasadmin.NewRepository(db)
	adminUC := saasadmin.NewUsecases(adminRepo)

	// Metering
	meteringRepo := saasmetering.NewRepository(db, nil)
	meteringMW := saasmetering.NewAPICallsMiddleware(meteringRepo)

	var jwtVerifier saasmiddleware.PrincipalVerifier
	if issuer := strings.TrimSpace(cfg.JWTIssuer); issuer != "" {
		identityUC := saasidentity.NewUsecasesWithOrgResolver(identityoidc.NewDiscoveryClient(issuer), orgRepo, saasidentity.Config{
			Issuer:      issuer,
			Audience:    strings.TrimSpace(cfg.JWTAudience),
			OrgClaim:    valueOrDefault(cfg.JWTOrgClaim, "org_id"),
			RoleClaim:   valueOrDefault(cfg.JWTRoleClaim, "role"),
			ScopesClaim: valueOrDefault(cfg.JWTScopesClaim, "scopes"),
			ActorClaim:  valueOrDefault(cfg.JWTActorClaim, "sub"),
		})
		jwtVerifier = &jwtPrincipalVerifier{uc: identityUC}
	}
	authMW := saasmiddleware.NewAuthMiddleware(jwtVerifier, &apiKeyPrincipalVerifier{uc: orgUC})

	// Billing — adapt admin.Usecases to billing.TenantSettingsPort
	billingRepo := saasbilling.NewRepository(db)
	stripeClient := saasbilling.NewStripeClient(cfg.StripeSecretKey)
	billingUC := saasbilling.NewUsecases(
		saasbilling.Config{
			StripeSecretKey:       cfg.StripeSecretKey,
			StripeWebhookSecret:   cfg.StripeWebhookSecret,
			StripePriceStarter:    cfg.StripePriceStarter,
			StripePriceGrowth:     cfg.StripePriceGrowth,
			StripePriceEnterprise: cfg.StripePriceEnterprise,
			TowerBaseURL:          cfg.TowerBaseURL,
		},
		billingRepo,
		&tenantSettingsAdapter{repo: adminRepo},
		stripeClient,
		nil, // notifications
		nil, // metrics
		logger,
	)
	billingHandler := saasbilling.NewHandler(billingUC)
	adminHandler := saasadmin.NewHandler(adminUC)

	cleanup := func() {
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return &SaaSServices{
		OrgHandler:     orgHandler,
		UsersHandler:   usersHandler,
		ClerkHandler:   clerkHandler,
		BillingHandler: billingHandler,
		AdminHandler:   adminHandler,
		AuthMiddleware: authMW,
		MeteringMW:     meteringMW,
		Cleanup:        cleanup,
	}, nil
}

// RegisterSaaSRoutes registers all saas-core HTTP handlers on the given mux.
func RegisterSaaSRoutes(mux *http.ServeMux, svc *SaaSServices) {
	if svc == nil {
		return
	}
	if svc.OrgHandler != nil {
		svc.OrgHandler.Register(mux)
	}
	if svc.UsersHandler != nil {
		svc.UsersHandler.Register(mux)
	}
	if svc.ClerkHandler != nil {
		svc.ClerkHandler.Register(mux)
	}
	if svc.BillingHandler != nil {
		svc.BillingHandler.Register(mux)
		svc.BillingHandler.RegisterWebhook(mux)
	}
	if svc.AdminHandler != nil {
		svc.AdminHandler.Register(mux)
	}
}

// --- Adapters to bridge interface mismatches between saas-core modules ---

// userSyncerAdapter adapts users.Usecases to clerkwebhook.UserSyncer.
type userSyncerAdapter struct {
	uc *saasusers.Usecases
}

func (a *userSyncerAdapter) SyncUser(ctx context.Context, externalID, email, name string, avatarURL *string) (saasclerk.SyncedUser, error) {
	u, err := a.uc.SyncUser(ctx, externalID, email, name, avatarURL)
	if err != nil {
		return saasclerk.SyncedUser{}, err
	}
	return saasclerk.SyncedUser{ID: u.ID, ExternalID: u.ExternalID}, nil
}

func (a *userSyncerAdapter) SyncOrganization(ctx context.Context, orgExternalID, orgName string) (uuid.UUID, error) {
	return a.uc.SyncOrganization(ctx, orgExternalID, orgName)
}

func (a *userSyncerAdapter) SyncMembership(ctx context.Context, orgID uuid.UUID, userExternalID, email, name string, avatarURL *string, role string) (saasclerk.SyncedMember, error) {
	m, err := a.uc.SyncMembership(ctx, orgID, userExternalID, email, name, avatarURL, role)
	if err != nil {
		return saasclerk.SyncedMember{}, err
	}
	return saasclerk.SyncedMember{ID: m.ID, OrgID: m.OrgID}, nil
}

func (a *userSyncerAdapter) SoftDeleteUser(ctx context.Context, externalID string) error {
	return a.uc.SoftDeleteUser(ctx, externalID)
}

func (a *userSyncerAdapter) RemoveMembership(ctx context.Context, userExternalID, orgExternalID, orgName string) error {
	return a.uc.RemoveMembership(ctx, userExternalID, orgExternalID, orgName)
}

// Verify users.Usecases has the methods we need (compile-time check).
var _ interface {
	SyncUser(context.Context, string, string, string, *string) (userdomain.User, error)
	SyncOrganization(context.Context, string, string) (uuid.UUID, error)
	SyncMembership(context.Context, uuid.UUID, string, string, string, *string, string) (userdomain.OrgMember, error)
	SoftDeleteUser(context.Context, string) error
	RemoveMembership(context.Context, string, string, string) error
} = (*saasusers.Usecases)(nil)

// tenantSettingsAdapter adapts admin.Usecases to billing.TenantSettingsPort.
type tenantSettingsAdapter struct {
	repo *saasadmin.Repository
}

func (a *tenantSettingsAdapter) UpsertTenantSettings(ctx context.Context, s billingdomain.TenantSettings) (billingdomain.TenantSettings, error) {
	stored, err := a.repo.UpsertTenantSettings(ctx, saasadmindomain.TenantSettings{
		OrgID:      s.OrgID,
		PlanCode:   s.PlanCode,
		Status:     saasadmindomain.TenantStatusActive,
		HardLimits: s.HardLimits,
		UpdatedAt:  s.UpdatedAt,
	})
	if err != nil {
		return billingdomain.TenantSettings{}, err
	}
	return billingdomain.TenantSettings{
		OrgID:      stored.OrgID,
		PlanCode:   stored.PlanCode,
		HardLimits: stored.HardLimits,
		UpdatedAt:  stored.UpdatedAt,
	}, nil
}

type apiKeyPrincipalVerifier struct {
	uc *saasorg.Usecases
}

func (v *apiKeyPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, sha256Hex(strings.TrimSpace(credential)))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		OrgID:      principal.OrgID.String(),
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "api_key",
	}, nil
}

type jwtPrincipalVerifier struct {
	uc *saasidentity.Usecases
}

func (v *jwtPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, strings.TrimSpace(credential))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		OrgID:      principal.OrgID.String(),
		Actor:      principal.Actor,
		Role:       principal.Role,
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "jwt",
	}, nil
}

func WrapAuth(handler http.Handler, nexusAuth *sharedapikey.Authenticator, svc *SaaSServices) http.Handler {
	if handler == nil {
		handler = http.NotFoundHandler()
	}

	nexusHandler := handler
	if nexusAuth != nil {
		nexusHandler = nexusAuth.Middleware(handler)
	}

	saasHandler := handler
	if svc != nil {
		if svc.MeteringMW != nil {
			saasHandler = svc.MeteringMW(saasHandler)
		}
		if svc.AuthMiddleware != nil {
			saasHandler = svc.AuthMiddleware(saasHandler)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case isSaaSPublicPath(r.URL.Path):
			handler.ServeHTTP(w, r)
		case isSaaSProtectedPath(r.URL.Path):
			if svc == nil || svc.AuthMiddleware == nil {
				handler.ServeHTTP(w, r)
				return
			}
			saasHandler.ServeHTTP(w, r)
		default:
			nexusHandler.ServeHTTP(w, r)
		}
	})
}

func isSaaSPublicPath(path string) bool {
	switch strings.TrimSpace(path) {
	case "/orgs", "/webhooks/clerk", "/v1/webhooks/stripe":
		return true
	default:
		return false
	}
}

func isSaaSProtectedPath(path string) bool {
	path = strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(path, "/admin"):
		return true
	case strings.HasPrefix(path, "/billing"):
		return true
	case strings.HasPrefix(path, "/users"):
		return true
	case strings.HasPrefix(path, "/orgs/"):
		return true
	default:
		return false
	}
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
