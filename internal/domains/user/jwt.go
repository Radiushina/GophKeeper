package user

import (
	"context"
	"fmt"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"
)

type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(secret string, ttl time.Duration) *JWT {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &JWT{secret: []byte(secret), ttl: ttl}
}

func (j *JWT) Generate(userID uuid.UUID) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signed, nil
}

func (j *JWT) Parse(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: parse jwt: %w", ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("%w: invalid token", ErrUnauthorized)
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("%w: invalid subject", ErrUnauthorized)
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: parse subject: %w", ErrUnauthorized, err)
	}

	return userID, nil
}

// HandleBearerAuth implements oas.SecurityHandler for note endpoints.
func (j *JWT) HandleBearerAuth(ctx context.Context, _ oas.OperationName, t oas.BearerAuth) (context.Context, error) {
	if t.Token == "" {
		return ctx, &ogenerrors.SecurityError{Err: ErrUnauthorized}
	}
	userID, err := j.Parse(t.Token)
	if err != nil {
		return ctx, &ogenerrors.SecurityError{Err: err}
	}
	return WithUserID(ctx, userID), nil
}
