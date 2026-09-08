package file

import (
	"context"
	"errors"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handler serves /api/v1/files.
type Handler struct {
	oas.FilesHandler

	service ServiceProvider
	log     *zap.Logger
}

// ServiceProvider is the file application port.
type ServiceProvider interface {
	Create(ctx context.Context, userID uuid.UUID, in CreateInput) (File, error)
	Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (File, error)
	Delete(ctx context.Context, userID, id uuid.UUID) (File, error)
	Get(ctx context.Context, userID, id uuid.UUID) (File, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]File, error)
}

// NewHandler builds a files HTTP handler.
func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

// FileCreate implements POST /api/v1/files.
func (h *Handler) FileCreate(ctx context.Context, req *oas.CreateFile) (oas.FileCreateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.FileCreateUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Create(ctx, userID, CreateInput{
		ID:               req.GetID(),
		Version:          req.GetVersion(),
		Nonce:            req.GetNonce(),
		Meta:             req.GetMeta().Value,
		Ciphertext:       req.GetCiphertext(),
		CiphertextSHA256: req.GetCiphertextSHA256(),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.FileCreateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.FileCreateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("file create", zap.Error(err))
			return &oas.FileCreateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// FileUpdate implements PUT /api/v1/files/{id}.
func (h *Handler) FileUpdate(ctx context.Context, req *oas.UpdateFile, params oas.FileUpdateParams) (oas.FileUpdateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.FileUpdateUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Update(ctx, userID, params.ID, UpdateInput{
		Version:          req.GetVersion(),
		Nonce:            req.GetNonce(),
		Meta:             req.GetMeta().Value,
		Ciphertext:       req.GetCiphertext(),
		CiphertextSHA256: req.GetCiphertextSHA256(),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.FileUpdateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.FileUpdateNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.FileUpdateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("file update", zap.Error(err))
			return &oas.FileUpdateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// FileDelete implements DELETE /api/v1/files/{id}.
func (h *Handler) FileDelete(ctx context.Context, params oas.FileDeleteParams) (oas.FileDeleteRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.FileDeleteUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Delete(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.FileDeleteNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.FileDeleteNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("file delete", zap.Error(err))
			return &oas.FileDeleteInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// FileGet implements GET /api/v1/files/{id}.
func (h *Handler) FileGet(ctx context.Context, params oas.FileGetParams) (oas.FileGetRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.FileGetUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Get(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound):
			return &oas.FileGetNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("file get", zap.Error(err))
			return &oas.FileGetInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// ListFiles implements GET /api/v1/files.
func (h *Handler) ListFiles(ctx context.Context, params oas.ListFilesParams) (oas.ListFilesRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.ListFilesUnauthorized{Msg: "user unauthorized"}, nil
	}
	var since *time.Time
	if params.Since.IsSet() {
		t := params.Since.Value
		since = &t
	}
	items, err := h.service.List(ctx, userID, since)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			return &oas.ListFilesBadRequest{Msg: "validate"}, nil
		}
		h.log.Error("file list", zap.Error(err))
		return &oas.ListFilesInternalServerError{Msg: "internal server error"}, nil
	}
	out := oas.FileListRes{Items: make([]oas.File, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, toOAS(items[i]))
	}
	return &out, nil
}
