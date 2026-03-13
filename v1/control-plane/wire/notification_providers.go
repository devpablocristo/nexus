package wire

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/wire"
	"github.com/rs/zerolog"

	"control-plane/cmd/config"
	"control-plane/internal/billing"
	"control-plane/internal/clerkwebhook"
	"control-plane/internal/incidents"
	"control-plane/internal/notifications"
)

func ProvideNotificationSender(cfg config.ServiceConfig, l zerolog.Logger) (notifications.EmailSender, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.NotificationBackend)) {
	case "", "noop":
		l.Info().Msg("notifications backend set to noop")
		return notifications.NewNoopSender(l), nil
	case "smtp":
		sender, err := notifications.NewSMTPSender(
			cfg.SMTPHost,
			cfg.SMTPPort,
			cfg.SMTPFromEmail,
			cfg.SMTPUsername,
			cfg.SMTPPassword,
		)
		if err != nil {
			return nil, err
		}
		l.Info().Str("host", cfg.SMTPHost).Int("port", cfg.SMTPPort).Msg("notifications backend set to smtp")
		return sender, nil
	case "ses":
		sender, err := notifications.NewSESSender(context.Background(), cfg.AWSRegion, cfg.SESFromEmail, cfg.SESFromName)
		if err != nil {
			return nil, err
		}
		l.Info().Str("region", cfg.AWSRegion).Msg("notifications backend set to ses")
		return sender, nil
	default:
		return nil, fmt.Errorf("unsupported NOTIFICATION_BACKEND: %s", cfg.NotificationBackend)
	}
}

func ProvideBillingNotificationPort(n *notifications.Usecases) billing.NotificationPort { return n }
func ProvideClerkWebhookNotificationPort(n *notifications.Usecases) clerkwebhook.NotificationPort {
	return n
}
func ProvideIncidentsNotificationPort(n *notifications.Usecases) incidents.NotificationPort { return n }

var NotificationsSet = wire.NewSet(
	notifications.NewRepository,
	ProvideNotificationSender,
	notifications.NewUsecases,
	notifications.NewHandler,
)
