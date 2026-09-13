package card

import (
	"context"
	"errors"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	oas.CardsHandler

	service ServiceProvider
	log     *zap.Logger
}

type ServiceProvider interface {
	Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Card, error)
	Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (Card, error)
	Delete(ctx context.Context, userID, id uuid.UUID) (Card, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Card, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Card, error)
}

func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

func (h *Handler) CardCreate(ctx context.Context, req *oas.CreateCard) (oas.CardCreateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.CardCreateUnauthorized{Msg: "user unauthorized"}, nil
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
			return &oas.CardCreateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.CardCreateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("card create", zap.Error(err))
			return &oas.CardCreateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

func (h *Handler) CardUpdate(ctx context.Context, req *oas.UpdateCard, params oas.CardUpdateParams) (oas.CardUpdateRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.CardUpdateUnauthorized{Msg: "user unauthorized"}, nil
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
			return &oas.CardUpdateBadRequest{Msg: "validate"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.CardUpdateNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrConflict):
			return &oas.CardUpdateConflict{Msg: "conflict"}, nil
		default:
			h.log.Error("card update", zap.Error(err))
			return &oas.CardUpdateInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

func (h *Handler) CardDelete(ctx context.Context, params oas.CardDeleteParams) (oas.CardDeleteRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.CardDeleteUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Delete(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			return &oas.CardDeleteNotFound{Msg: "not found"}, nil
		case errors.Is(err, ErrNotFound):
			return &oas.CardDeleteNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("card delete", zap.Error(err))
			return &oas.CardDeleteInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

func (h *Handler) CardGet(ctx context.Context, params oas.CardGetParams) (oas.CardGetRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.CardGetUnauthorized{Msg: "user unauthorized"}, nil
	}
	n, err := h.service.Get(ctx, userID, params.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound):
			return &oas.CardGetNotFound{Msg: "not found"}, nil
		default:
			h.log.Error("card get", zap.Error(err))
			return &oas.CardGetInternalServerError{Msg: "internal server error"}, nil
		}
	}
	out := toOAS(n)
	return &out, nil
}

func (h *Handler) ListCards(ctx context.Context, params oas.ListCardsParams) (oas.ListCardsRes, error) {
	userID, err := user.IDFromContext(ctx)
	if err != nil {
		return &oas.ListCardsUnauthorized{Msg: "user unauthorized"}, nil
	}
	var since *time.Time
	if params.Since.IsSet() {
		t := params.Since.Value
		since = &t
	}
	items, err := h.service.List(ctx, userID, since)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			return &oas.ListCardsBadRequest{Msg: "validate"}, nil
		}
		h.log.Error("card list", zap.Error(err))
		return &oas.ListCardsInternalServerError{Msg: "internal server error"}, nil
	}
	out := oas.CardListRes{Items: make([]oas.Card, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, toOAS(items[i]))
	}
	return &out, nil
}
