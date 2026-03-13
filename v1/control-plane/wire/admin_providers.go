package wire

import (
	"github.com/google/wire"

	"control-plane/internal/admin"
	"control-plane/internal/notifications"
)

func ProvideAdminUsecases(repo admin.RepositoryPort, notificationsUC *notifications.Usecases) *admin.Usecases {
	return admin.NewUsecasesWithNotifications(repo, notificationsUC)
}

var AdminSet = wire.NewSet(
	admin.NewRepository,
	wire.Bind(new(admin.RepositoryPort), new(*admin.Repository)),
	ProvideAdminUsecases,
	admin.NewHandler,
)
