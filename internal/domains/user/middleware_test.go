package user_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewAuthMiddleware(t *testing.T) {
	t.Parallel()

	jwtProvider := user.NewJWT("test-secret", time.Hour)
	userID := uuid.New()

	validToken, err := jwtProvider.Generate(userID)
	require.NoError(t, err)

	handler := user.NewAuthMiddleware(jwtProvider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/secret" {
			got, err := user.IDFromContext(r.Context())
			require.NoError(t, err)
			require.Equal(t, userID, got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		method         string
		path           string
		authHeader     string
		wantStatus     int
		wantBodySubstr string
	}{
		{
			name:       "success",
			method:     http.MethodGet,
			path:       "/api/secret",
			authHeader: "Bearer " + validToken,
			wantStatus: http.StatusOK,
		},
		{
			name:           "missing header",
			method:         http.MethodGet,
			path:           "/api/secret",
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
		{
			name:           "invalid token",
			method:         http.MethodGet,
			path:           "/api/secret",
			authHeader:     "Bearer invalid",
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
		{
			name:           "invalid scheme",
			method:         http.MethodGet,
			path:           "/api/secret",
			authHeader:     "Basic " + validToken,
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
		{
			name:       "public register without token",
			method:     http.MethodPost,
			path:       "/api/user/register",
			wantStatus: http.StatusOK,
		},
		{
			name:       "public login without token",
			method:     http.MethodPost,
			path:       "/api/user/login",
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantBodySubstr != "" {
				require.Contains(t, rec.Body.String(), tc.wantBodySubstr)
			}
		})
	}
}
