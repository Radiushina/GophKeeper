package di

import (
	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/wire"
)

var (
	ConfigSet = wire.NewSet(
		providers.NewConfig,
		providers.NewLogger,
	)

	InfraSet = wire.NewSet(
		providers.NewPostgres,
		providers.NewJWT,
		user.NewHasher,
		user.NewRepository,
		wire.Bind(new(user.RepoProvider), new(*user.UsersRepo)),
		wire.Bind(new(user.TokenProvider), new(*user.JWT)),
		wire.Bind(new(user.HasherProvider), new(*user.Hasher)),
	)

	HandlerSet = wire.NewSet(
		user.NewHandler,
	)

	ServerSet = wire.NewSet(
		providers.NewHTTPServer,
		providers.NewServers,
	)

	ServicesSet = wire.NewSet(
		user.NewService,
		wire.Bind(new(user.ServiceProvider), new(*user.Service)),
	)
)

var AllSets = wire.NewSet(
	ConfigSet,
	InfraSet,
	HandlerSet,
	ServerSet,
	ServicesSet,
)
