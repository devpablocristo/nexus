package wire

import (
	"github.com/google/wire"

	"control-plane/internal/clerkwebhook"
)

var ClerkWebhookSet = wire.NewSet(
	ProvideClerkWebhookNotificationPort,
	clerkwebhook.NewHandler,
)
