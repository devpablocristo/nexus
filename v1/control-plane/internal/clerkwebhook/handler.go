package clerkwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"control-plane/cmd/config"
	saasmetrics "control-plane/internal/shared/metrics"
	"control-plane/internal/users"
)

const (
	headerSvixID        = "svix-id"
	headerSvixTimestamp = "svix-timestamp"
	headerSvixSignature = "svix-signature"

	maxWebhookBodyBytes = 2 * 1024 * 1024
	maxClockSkew        = 5 * time.Minute

	webhookRateLimit  = 60
	webhookRateWindow = 1 * time.Minute
)

var sigV1Regexp = regexp.MustCompile(`v1,([A-Za-z0-9+/=_-]+)`)

type Handler struct {
	uc            *users.Usecases
	notifications NotificationPort
	towerBaseURL  string
	webhookSecret string
	now           func() time.Time
	logger        zerolog.Logger

	rateMu    sync.Mutex
	rateCount int
	rateReset time.Time
}

type NotificationPort interface {
	NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}

func NewHandler(cfg config.ServiceConfig, uc *users.Usecases, notifications NotificationPort, l zerolog.Logger) *Handler {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.TowerBaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	return &Handler{
		uc:            uc,
		notifications: notifications,
		towerBaseURL:  baseURL,
		webhookSecret: strings.TrimSpace(cfg.ClerkWebhookSecret),
		now:           time.Now,
		logger:        l,
	}
}

func (h *Handler) checkRateLimit() bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	now := h.now()
	if now.After(h.rateReset) {
		h.rateCount = 0
		h.rateReset = now.Add(webhookRateWindow)
	}
	h.rateCount++
	return h.rateCount <= webhookRateLimit
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/webhooks/clerk", h.handle)
}

func (h *Handler) handle(c *gin.Context) {
	if h.webhookSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "CONFIG_ERROR", "message": "CLERK_WEBHOOK_SECRET not configured"},
		})
		return
	}
	if !h.checkRateLimit() {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{"code": "RATE_LIMIT", "message": "webhook rate limit exceeded"},
		})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid body"}})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	if err := verifySvix(
		h.webhookSecret,
		c.GetHeader(headerSvixID),
		c.GetHeader(headerSvixTimestamp),
		c.GetHeader(headerSvixSignature),
		body,
		h.now,
	); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "invalid webhook signature"}})
		return
	}

	var evt clerkEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid webhook payload"}})
		return
	}
	saasmetrics.WebhooksReceived.WithLabelValues("clerk", strings.TrimSpace(evt.Type)).Inc()

	if err := h.dispatch(c.Request.Context(), evt); err != nil {
		h.logger.Error().Err(err).Str("type", evt.Type).Msg("failed processing clerk webhook")
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed processing webhook"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) dispatch(ctx context.Context, evt clerkEventEnvelope) error {
	switch strings.TrimSpace(evt.Type) {
	case "user.created":
		return h.onUserUpsert(ctx, evt.Data, true)
	case "user.updated":
		return h.onUserUpsert(ctx, evt.Data, false)
	case "user.deleted":
		return h.onUserDeleted(ctx, evt.Data)
	case "organization.created":
		return h.onOrganizationCreated(ctx, evt.Data)
	case "organizationMembership.created":
		return h.onOrganizationMembershipCreated(ctx, evt.Data)
	case "organizationMembership.deleted":
		return h.onOrganizationMembershipDeleted(ctx, evt.Data)
	default:
		return nil
	}
}

func (h *Handler) onUserUpsert(ctx context.Context, raw json.RawMessage, sendWelcome bool) error {
	var data clerkUserData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	email, err := users.BuildWebhookUserEmail(data.primaryEmail(), data.allEmails())
	if err != nil {
		return err
	}
	name := users.FormatUserName(data.FirstName, data.LastName, email)
	_, err = h.uc.SyncUser(ctx, data.ID, email, name, nullable(data.ImageURL))
	if err != nil {
		return err
	}
	if sendWelcome && h.notifications != nil {
		payload := map[string]string{
			"recipient_name":  name,
			"action_url":      h.towerBaseURL + "/tools",
			"preferences_url": h.towerBaseURL + "/settings/notifications",
		}
		go func(userExternalID string, data map[string]string) {
			if notifyErr := h.notifications.NotifyUser(context.Background(), userExternalID, "welcome", data); notifyErr != nil {
				h.logger.Error().Err(notifyErr).Str("user_external_id", userExternalID).Msg("failed async welcome notification")
			}
		}(data.ID, payload)
	}
	return nil
}

func (h *Handler) onUserDeleted(ctx context.Context, raw json.RawMessage) error {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if strings.TrimSpace(data.ID) == "" {
		return errors.New("user.deleted: missing user id")
	}
	return h.uc.SoftDeleteUser(ctx, data.ID)
}

func (h *Handler) onOrganizationMembershipDeleted(ctx context.Context, raw json.RawMessage) error {
	var data clerkMembershipData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	userID := firstNonEmpty(data.PublicUserData.UserID, data.User.ID)
	orgName := firstNonEmpty(data.Organization.Name, data.Organization.Slug, data.Organization.ID)
	if userID == "" || orgName == "" {
		return errors.New("organizationMembership.deleted: missing user_id or org")
	}
	return h.uc.RemoveMembership(ctx, userID, orgName)
}

func (h *Handler) onOrganizationCreated(ctx context.Context, raw json.RawMessage) error {
	var data clerkOrganizationData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	orgName := firstNonEmpty(data.Name, data.Slug, data.ID)
	if orgName == "" {
		return errors.New("organization name is empty")
	}
	_, err := h.uc.SyncOrganization(ctx, orgName)
	return err
}

func (h *Handler) onOrganizationMembershipCreated(ctx context.Context, raw json.RawMessage) error {
	var data clerkMembershipData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	orgName := firstNonEmpty(data.Organization.Name, data.Organization.Slug, data.Organization.ID)
	if orgName == "" {
		return errors.New("organization name is empty")
	}
	orgID, err := h.uc.SyncOrganization(ctx, orgName)
	if err != nil {
		return err
	}

	userID := firstNonEmpty(data.PublicUserData.UserID, data.User.ID)
	if userID == "" {
		return errors.New("membership user_id missing")
	}
	email, err := users.BuildWebhookUserEmail(
		data.User.primaryEmail(),
		[]string{data.PublicUserData.Identifier},
	)
	if err != nil {
		return err
	}
	firstName := firstNonEmpty(data.PublicUserData.FirstName, data.User.FirstName)
	lastName := firstNonEmpty(data.PublicUserData.LastName, data.User.LastName)
	imageURL := firstNonEmpty(data.PublicUserData.ImageURL, data.User.ImageURL)
	name := users.FormatUserName(firstName, lastName, email)

	_, err = h.uc.SyncMembership(
		ctx,
		orgID,
		userID,
		email,
		name,
		nullable(imageURL),
		data.Role,
	)
	return err
}

type clerkEventEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkEmailAddress struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

type clerkUserData struct {
	ID                    string              `json:"id"`
	FirstName             string              `json:"first_name"`
	LastName              string              `json:"last_name"`
	ImageURL              string              `json:"image_url"`
	PrimaryEmailAddressID string              `json:"primary_email_address_id"`
	EmailAddresses        []clerkEmailAddress `json:"email_addresses"`
}

func (u clerkUserData) primaryEmail() string {
	if u.PrimaryEmailAddressID != "" {
		for _, email := range u.EmailAddresses {
			if strings.TrimSpace(email.ID) == strings.TrimSpace(u.PrimaryEmailAddressID) {
				return strings.TrimSpace(email.EmailAddress)
			}
		}
	}
	for _, email := range u.EmailAddresses {
		if s := strings.TrimSpace(email.EmailAddress); s != "" {
			return s
		}
	}
	return ""
}

func (u clerkUserData) allEmails() []string {
	out := make([]string, 0, len(u.EmailAddresses))
	for _, email := range u.EmailAddresses {
		out = append(out, strings.TrimSpace(email.EmailAddress))
	}
	return out
}

type clerkOrganizationData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type clerkMembershipData struct {
	ID           string `json:"id"`
	Role         string `json:"role"`
	Organization struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"organization"`
	PublicUserData struct {
		UserID     string `json:"user_id"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Identifier string `json:"identifier"`
		ImageURL   string `json:"image_url"`
	} `json:"public_user_data"`
	User clerkUserData `json:"user"`
}

// verifySvix performs Svix webhook signature verification per
// https://docs.svix.com/receiving/verifying-payloads/how-manual
// Intentionally avoids the svix-webhooks SDK to keep the dependency tree minimal.
func verifySvix(secret, id, ts, sigHeader string, payload []byte, now func() time.Time) error {
	secret = strings.TrimSpace(secret)
	id = strings.TrimSpace(id)
	ts = strings.TrimSpace(ts)
	sigHeader = strings.TrimSpace(sigHeader)
	if secret == "" || id == "" || ts == "" || sigHeader == "" {
		return errors.New("missing svix headers")
	}

	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid svix timestamp")
	}
	delta := now().UTC().Sub(time.Unix(timestamp, 0).UTC())
	if delta < 0 {
		delta = -delta
	}
	if delta > maxClockSkew {
		return errors.New("svix timestamp expired")
	}

	secretBytes, err := decodeSvixSecret(secret)
	if err != nil {
		return err
	}

	message := id + "." + ts + "." + string(payload)
	mac := hmac.New(sha256.New, secretBytes)
	_, _ = mac.Write([]byte(message))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	signatures := extractV1Signatures(sigHeader)
	if len(signatures) == 0 {
		return errors.New("missing v1 signatures")
	}
	for _, candidate := range signatures {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return nil
		}
	}
	return errors.New("signature mismatch")
}

func decodeSvixSecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	secret = strings.TrimPrefix(secret, "whsec_")
	if secret == "" {
		return nil, errors.New("invalid svix secret")
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		b, err := enc.DecodeString(secret)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, errors.New("invalid svix secret encoding")
}

func extractV1Signatures(raw string) []string {
	matches := sigV1Regexp.FindAllStringSubmatch(raw, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			out = append(out, strings.TrimSpace(match[1]))
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func nullable(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
