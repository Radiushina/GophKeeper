package providers

import (
	"context"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
)

// OASHandler joins auth, note, card and file HTTP handlers into one ogen Handler.
type OASHandler struct {
	users *user.Handler
	notes *note.Handler
	cards *card.Handler
	files *file.Handler
}

// NewOASHandler composes user, note, card and file handlers.
func NewOASHandler(users *user.Handler, notes *note.Handler, cards *card.Handler, files *file.Handler) *OASHandler {
	return &OASHandler{users: users, notes: notes, cards: cards, files: files}
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

// CardCreate implements POST /api/v1/cards.
func (h *OASHandler) CardCreate(ctx context.Context, req *oas.CreateCard) (oas.CardCreateRes, error) {
	return h.cards.CardCreate(ctx, req)
}

// CardUpdate implements PUT /api/v1/cards/{id}.
func (h *OASHandler) CardUpdate(ctx context.Context, req *oas.UpdateCard, params oas.CardUpdateParams) (oas.CardUpdateRes, error) {
	return h.cards.CardUpdate(ctx, req, params)
}

// CardDelete implements DELETE /api/v1/cards/{id}.
func (h *OASHandler) CardDelete(ctx context.Context, params oas.CardDeleteParams) (oas.CardDeleteRes, error) {
	return h.cards.CardDelete(ctx, params)
}

// CardGet implements GET /api/v1/cards/{id}.
func (h *OASHandler) CardGet(ctx context.Context, params oas.CardGetParams) (oas.CardGetRes, error) {
	return h.cards.CardGet(ctx, params)
}

// ListCards implements GET /api/v1/cards.
func (h *OASHandler) ListCards(ctx context.Context, params oas.ListCardsParams) (oas.ListCardsRes, error) {
	return h.cards.ListCards(ctx, params)
}

// FileCreate implements POST /api/v1/files.
func (h *OASHandler) FileCreate(ctx context.Context, req *oas.CreateFile) (oas.FileCreateRes, error) {
	return h.files.FileCreate(ctx, req)
}

// FileUpdate implements PUT /api/v1/files/{id}.
func (h *OASHandler) FileUpdate(ctx context.Context, req *oas.UpdateFile, params oas.FileUpdateParams) (oas.FileUpdateRes, error) {
	return h.files.FileUpdate(ctx, req, params)
}

// FileDelete implements DELETE /api/v1/files/{id}.
func (h *OASHandler) FileDelete(ctx context.Context, params oas.FileDeleteParams) (oas.FileDeleteRes, error) {
	return h.files.FileDelete(ctx, params)
}

// FileGet implements GET /api/v1/files/{id}.
func (h *OASHandler) FileGet(ctx context.Context, params oas.FileGetParams) (oas.FileGetRes, error) {
	return h.files.FileGet(ctx, params)
}

// ListFiles implements GET /api/v1/files.
func (h *OASHandler) ListFiles(ctx context.Context, params oas.ListFilesParams) (oas.ListFilesRes, error) {
	return h.files.ListFiles(ctx, params)
}
