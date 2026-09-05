package user

import (
	"errors"

	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists") // 409
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCredentials = errors.New("invalid credentials") //401
)

type User struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Login          string    `db:"login" json:"login"`
	Password       string    `db:"password" json:"-"`
	KdfSalt        []byte    `db:"kdf_salt" json:"-"`
	KdfMemory      int       `db:"kdf_memory" json:"-"`
	KdfIterations  int       `db:"kdf_iterations" json:"-"`
	KdfParallelism int       `db:"kdf_parallelism" json:"-"`
	KdfVersion     int       `db:"kdf_version" json:"-"`
	ProtectedKey   []byte    `db:"protected_key" json:"-"`
	KeyHash        []byte    `db:"key_hash" json:"-"`
}

// RegisterInput is client register payload (vault blobs opaque to the server).
type RegisterInput struct {
	Login        string
	Password     string
	KdfSalt      []byte
	ProtectedKey []byte
	KeyHash      []byte
}

type UserResponse struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
}

type AuthUserResponse struct {
	User         UserResponse
	Token        string
	KdfSalt      []byte
	KdfParams    vault.Params
	ProtectedKey []byte
	KeyHash      []byte
}

func NewAuthSession(u User, token string) AuthUserResponse {
	return AuthUserResponse{
		User: UserResponse{
			ID:    u.ID,
			Login: u.Login,
		},
		Token:   token,
		KdfSalt: append([]byte(nil), u.KdfSalt...),
		KdfParams: vault.Params{
			Memory:      uint32(u.KdfMemory),
			Iterations:  uint32(u.KdfIterations),
			Parallelism: uint8(u.KdfParallelism),
			Version:     uint8(u.KdfVersion),
		},
		ProtectedKey: append([]byte(nil), u.ProtectedKey...),
		KeyHash:      append([]byte(nil), u.KeyHash...),
	}
}
