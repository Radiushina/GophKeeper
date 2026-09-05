package di

import (
	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
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
		note.NewRepository,
		wire.Bind(new(user.RepoProvider), new(*user.UsersRepo)),
		wire.Bind(new(note.RepoProvider), new(*note.NotesRepo)),
		wire.Bind(new(user.TokenProvider), new(*user.JWT)),
		wire.Bind(new(user.HasherProvider), new(*user.Hasher)),
	)

	HandlerSet = wire.NewSet(
		user.NewHandler,
		note.NewHandler,
		providers.NewOASHandler,
		wire.Bind(new(oas.Handler), new(*providers.OASHandler)),
	)

	ServerSet = wire.NewSet(
		providers.NewHTTPServer,
		providers.NewServers,
	)

	ServicesSet = wire.NewSet(
		user.NewService,
		note.NewService,
		wire.Bind(new(user.ServiceProvider), new(*user.Service)),
		wire.Bind(new(note.ServiceProvider), new(*note.Service)),
	)
)

var AllSets = wire.NewSet(
	ConfigSet,
	InfraSet,
	HandlerSet,
	ServerSet,
	ServicesSet,
)
