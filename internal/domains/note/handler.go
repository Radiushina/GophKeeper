package note

import (
	"context"
	"errors"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handler serves /api/v1/notes.
type Handler struct {
	oas.NotesHandler

	service ServiceProvider
	log     *zap.Logger
}

// ServiceProvider is the note application port.
type ServiceProvider interface {
	Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Note, error)
	Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (Note, error)
	Delete(ctx context.Context, userID, id uuid.UUID) (Note, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Note, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Note, error)
}

// NewHandler builds a notes HTTP handler.
func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

// NoteCreate implements POST /api/v1/notes.
func (h *Handler) NoteCreate(ctx context.Context, req *oas.CreateNote) (oas.NoteCreateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.NoteCreateUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Create(ctx, userID, CreateInput{
		ID:               req.GetID(),
		Version:          req.GetVersion(),
		Nonce:            req.GetNonce(),
		Ciphertext:       req.GetCiphertext(),
		CiphertextSHA256: req.GetCiphertextSHA256(),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.NoteCreateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.NoteCreateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("note create", zap.Error(err))
			return &oas.NoteCreateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// NoteUpdate implements PUT /api/v1/notes/{id}.
func (h *Handler) NoteUpdate(ctx context.Context, req *oas.UpdateNote, params oas.NoteUpdateParams) (oas.NoteUpdateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.NoteUpdateUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Update(ctx, userID, params.ID, UpdateInput{
		Version:          req.GetVersion(),
		Nonce:            req.GetNonce(),
		Ciphertext:       req.GetCiphertext(),
		CiphertextSHA256: req.GetCiphertextSHA256(),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.NoteUpdateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.NoteUpdateNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.NoteUpdateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("note update", zap.Error(err))
			return &oas.NoteUpdateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// NoteDelete implements DELETE /api/v1/notes/{id}.
func (h *Handler) NoteDelete(ctx context.Context, params oas.NoteDeleteParams) (oas.NoteDeleteRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.NoteDeleteUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Delete(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.NoteDeleteNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.NoteDeleteNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("note delete", zap.Error(err))
			return &oas.NoteDeleteInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// NoteGet implements GET /api/v1/notes/{id}.
func (h *Handler) NoteGet(ctx context.Context, params oas.NoteGetParams) (oas.NoteGetRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.NoteGetUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Get(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound):
			return &oas.NoteGetNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("note get", zap.Error(err))
			return &oas.NoteGetInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

// ListNotes implements GET /api/v1/notes.
func (h *Handler) ListNotes(ctx context.Context, params oas.ListNotesParams) (oas.ListNotesRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.ListNotesUnauthorized{Msg: "user unauthorized"}, nil
	}
	var since *time.Time
	if params.Since.IsSet() {
		t := params.Since.Value
		since = &t
	}
	items, err := h.service.List(ctx, userID, since)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			return &oas.ListNotesBadRequest{Msg: "validate"}, nil
		}
		h.log.Error("note list", zap.Error(err))
		return &oas.ListNotesInternalServerError{Msg: "internal server error"}, nil
	}
	out := oas.NoteListRes{Items: make([]oas.Note, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, toOAS(items[i]))
	}
	return &out, nil
}
