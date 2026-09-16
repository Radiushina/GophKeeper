package di

import (
	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/blob"
	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
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
		blob.NewS3,
		user.NewHasher,
		user.NewRepository,
		note.NewRepository,
		card.NewRepository,
		file.NewRepository,
		wire.Bind(new(blob.Store), new(*blob.S3)),
		wire.Bind(new(user.RepoProvider), new(*user.UsersRepo)),
		wire.Bind(new(note.RepoProvider), new(*note.NotesRepo)),
		wire.Bind(new(card.RepoProvider), new(*card.CardsRepo)),
		wire.Bind(new(file.RepoProvider), new(*file.FilesRepo)),
		wire.Bind(new(user.TokenProvider), new(*user.JWT)),
		wire.Bind(new(user.HasherProvider), new(*user.Hasher)),
	)

	HandlerSet = wire.NewSet(
		user.NewHandler,
		note.NewHandler,
		card.NewHandler,
		file.NewHandler,
		providers.NewOASHandler,
		wire.Bind(new(oas.Handler), new(*providers.OASHandler)),
	)

	ServerSet = wire.NewSet(
		providers.NewHTTPServer,
		providers.NewGRPCServer,
		providers.NewServers,
	)

	ServicesSet = wire.NewSet(
		user.NewService,
		note.NewService,
		card.NewService,
		file.NewService,
		wire.Bind(new(user.ServiceProvider), new(*user.Service)),
		wire.Bind(new(note.ServiceProvider), new(*note.Service)),
		wire.Bind(new(card.ServiceProvider), new(*card.Service)),
		wire.Bind(new(file.ServiceProvider), new(*file.Service)),
	)
)

var AllSets = wire.NewSet(
	ConfigSet,
	InfraSet,
	HandlerSet,
	ServerSet,
	ServicesSet,
)
