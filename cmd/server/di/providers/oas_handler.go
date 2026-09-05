package providers

import (
	"context"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
)

// OASHandler joins auth and note HTTP handlers into one ogen Handler.
type OASHandler struct {
	users *user.Handler
	notes *note.Handler
}

// NewOASHandler composes user and note handlers.
func NewOASHandler(users *user.Handler, notes *note.Handler) *OASHandler {
	return &OASHandler{users: users, notes: notes}
}

// AuthRegister implements POST /api/v1/user/register.
func (h *OASHandler) AuthRegister(ctx context.Context, req *oas.RegisterReq) (oas.AuthRegisterRes, error) {
	return h.users.AuthRegister(ctx, req)
}

// AuthLogin implements POST /api/v1/user/login.
func (h *OASHandler) AuthLogin(ctx context.Context, req *oas.AuthLoginReq) (oas.AuthLoginRes, error) {
	return h.users.AuthLogin(ctx, req)
}

// NoteCreate implements POST /api/v1/notes.
func (h *OASHandler) NoteCreate(ctx context.Context, req *oas.CreateNote) (oas.NoteCreateRes, error) {
	return h.notes.NoteCreate(ctx, req)
}

// NoteUpdate implements PUT /api/v1/notes/{id}.
func (h *OASHandler) NoteUpdate(ctx context.Context, req *oas.UpdateNote, params oas.NoteUpdateParams) (oas.NoteUpdateRes, error) {
	return h.notes.NoteUpdate(ctx, req, params)
}

// NoteDelete implements DELETE /api/v1/notes/{id}.
func (h *OASHandler) NoteDelete(ctx context.Context, params oas.NoteDeleteParams) (oas.NoteDeleteRes, error) {
	return h.notes.NoteDelete(ctx, params)
}

// NoteGet implements GET /api/v1/notes/{id}.
func (h *OASHandler) NoteGet(ctx context.Context, params oas.NoteGetParams) (oas.NoteGetRes, error) {
	return h.notes.NoteGet(ctx, params)
}

// ListNotes implements GET /api/v1/notes.
func (h *OASHandler) ListNotes(ctx context.Context, params oas.ListNotesParams) (oas.ListNotesRes, error) {
	return h.notes.ListNotes(ctx, params)
}
