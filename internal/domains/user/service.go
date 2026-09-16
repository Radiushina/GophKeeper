package user

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

type (
	Service struct {
		repo   RepoProvider
		tokens TokenProvider
		hasher HasherProvider
	}

	RepoProvider interface {
		CreateUser(ctx context.Context, u User) (User, error)
		GetByLogin(ctx context.Context, login string) (User, error)
	}

	TokenProvider interface {
		Generate(userID uuid.UUID) (string, error)
	}

	HasherProvider interface {
		Hash(plain string) (string, error)
		Compare(hash, plain string) error
	}
)

func NewService(repo RepoProvider, tokens TokenProvider, hasher HasherProvider) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		hasher: hasher,
	}
}

func (s *Service) CreateUser(ctx context.Context, in RegisterInput) (AuthUserResponse, error) {
	if err := validateRegisterInput(in); err != nil {
		return AuthUserResponse{}, fmt.Errorf("%w: %w", ErrInvalidCredentials, err)
	}

	hashed, err := s.hasher.Hash(in.Password)
	if err != nil {
		return AuthUserResponse{}, fmt.Errorf("hash password: %w", err)
	}

	p := vault.DefaultParams()
	created, err := s.repo.CreateUser(ctx, User{
		Login:          in.Login,
		Password:       hashed,
		KdfSalt:        append([]byte(nil), in.KdfSalt...),
		KdfMemory:      int(p.Memory),
		KdfIterations:  int(p.Iterations),
		KdfParallelism: int(p.Parallelism),
		KdfVersion:     int(p.Version),
		ProtectedKey:   append([]byte(nil), in.ProtectedKey...),
		KeyHash:        append([]byte(nil), in.KeyHash...),
	})
	if err != nil {
		return AuthUserResponse{}, fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Generate(created.ID)
	if err != nil {
		return AuthUserResponse{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(created, token), nil
}

func (s *Service) GetByLogin(ctx context.Context, login, password string) (AuthUserResponse, error) {
	if login == "" || password == "" {
		return AuthUserResponse{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	u, err := s.repo.GetByLogin(ctx, login)
	if err != nil {
		return AuthUserResponse{}, fmt.Errorf("authenticate: %w", err)
	}

	if err := s.hasher.Compare(u.Password, password); err != nil {
		return AuthUserResponse{}, ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(u.ID)
	if err != nil {
		return AuthUserResponse{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(u, token), nil
}

func validateRegisterInput(in RegisterInput) error {
	if in.Login == "" || in.Password == "" {
		return fmt.Errorf("login and password are required")
	}
	if len(in.KdfSalt) != vault.SaltSize {
		return fmt.Errorf("kdf_salt must be %d bytes", vault.SaltSize)
	}
	if len(in.KeyHash) != sha256.Size {
		return fmt.Errorf("key_hash must be %d bytes", sha256.Size)
	}
	// nonce (12) + AES-GCM tag (16) + at least 1 byte of sealed VK
	if len(in.ProtectedKey) < vault.NonceSize+16+1 {
		return fmt.Errorf("protected_key is too short")
	}
	return nil
}
