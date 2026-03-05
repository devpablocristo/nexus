package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/clerkwebhook"
)

var ClerkWebhookSet = wire.NewSet(
	clerkwebhook.NewHandler,
)
