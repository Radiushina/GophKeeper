package providers

import (
	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
)

func NewJWT(cfg *config.Config) *user.JWT {
	return user.NewJWT(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)
}
