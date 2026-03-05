package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/rs/zerolog"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

type NoopSender struct {
	logger zerolog.Logger
}

func NewNoopSender(logger zerolog.Logger) *NoopSender {
	return &NoopSender{logger: logger}
}

func (s *NoopSender) Send(_ context.Context, to, subject, _ string, _ string) error {
	s.logger.Info().Str("to", strings.TrimSpace(to)).Str("subject", strings.TrimSpace(subject)).Msg("notifications noop sender: email suppressed")
	return nil
}

type SESSender struct {
	client   *ses.Client
	fromAddr string
}

func NewSESSender(ctx context.Context, region, fromEmail, fromName string) (*SESSender, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil, errors.New("AWS_REGION is required for ses notification backend")
	}
	fromEmail = strings.TrimSpace(fromEmail)
	if fromEmail == "" {
		return nil, errors.New("AWS_SES_FROM_EMAIL is required for ses notification backend")
	}
	cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	fromName = strings.TrimSpace(fromName)
	fromAddr := fromEmail
	if fromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	return &SESSender{
		client:   ses.NewFromConfig(cfg),
		fromAddr: fromAddr,
	}, nil
}

func (s *SESSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	if to == "" {
		return errors.New("recipient email is required")
	}
	if subject == "" {
		return errors.New("subject is required")
	}
	if textBody == "" {
		textBody = "Please open this email in an HTML-capable client."
	}
	_, err := s.client.SendEmail(ctx, &ses.SendEmailInput{
		Source: &s.fromAddr,
		Destination: &sestypes.Destination{
			ToAddresses: []string{to},
		},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{Data: &subject, Charset: strPtr("UTF-8")},
			Body: &sestypes.Body{
				Html: &sestypes.Content{Data: &htmlBody, Charset: strPtr("UTF-8")},
				Text: &sestypes.Content{Data: &textBody, Charset: strPtr("UTF-8")},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}

func strPtr(v string) *string {
	return &v
}
