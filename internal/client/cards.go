package client

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

// CardPlain is the decrypted card secret. Meta on the wire is plaintext.
type CardPlain struct {
	Number string `json:"number"`
	Holder string `json:"holder"`
	Expiry string `json:"expiry"`
	CVV    string `json:"cvv"`
}

// CardAdd encrypts a card and creates it on the server.
func CardAdd(ctx context.Context, app *App, plain CardPlain, meta string) error {
	if strings.TrimSpace(plain.Number) == "" {
		return fmt.Errorf("number is required")
	}
	blob, err := sealCard(app, plain)
	if err != nil {
		return err
	}
	res, err := app.Client.CardCreate(ctx, &oas.CreateCard{
		ID:               uuid.New(),
		Version:          card.CreateVersion,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, os.Stdout)
}

// CardUpdate encrypts a replacement blob and writes it with optimistic locking.
// All card fields are required (no partial merge).
func CardUpdate(ctx context.Context, app *App, id uuid.UUID, version int64, plain CardPlain, meta string) error {
	if strings.TrimSpace(plain.Number) == "" {
		return fmt.Errorf("number is required")
	}
	if version == 0 {
		current, err := fetchCard(ctx, app, id)
		if err != nil {
			return err
		}
		version = current.Version
	}
	blob, err := sealCard(app, plain)
	if err != nil {
		return err
	}
	res, err := app.Client.CardUpdate(ctx, &oas.UpdateCard{
		Version:          version,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	}, oas.CardUpdateParams{ID: id})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, os.Stdout)
}

// CardDelete tombstones a card on the server.
func CardDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.CardDelete(ctx, oas.CardDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, os.Stdout)
}

// CardGet fetches and decrypts one card.
func CardGet(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.CardGet(ctx, oas.CardGetParams{ID: id})
	if err != nil {
		if isOffline(err) {
			return writeCachedCard(app, id, os.Stdout)
		}
		return err
	}
	return handleCardRes(app, res, os.Stdout)
}

// CardList prints decrypted cards. since is the optional sync cursor.
func CardList(ctx context.Context, app *App, since *time.Time) error {
	return writeCardList(ctx, app, since, os.Stdout)
}

func writeCardList(ctx context.Context, app *App, since *time.Time, w io.Writer) error {
	params := oas.ListCardsParams{}
	if since != nil {
		params.Since = oas.NewOptDateTime(*since)
	}
	res, err := app.Client.ListCards(ctx, params)
	if err != nil {
		if isOffline(err) {
			return writeCachedCards(app, w)
		}
		return err
	}
	switch v := res.(type) {
	case *oas.CardListRes:
		live := make([]cachedCard, 0, len(v.Items))
		for i := range v.Items {
			if err := printCard(app, w, v.Items[i]); err != nil {
				return err
			}
			if n, ok := toCachedCard(app, v.Items[i]); ok {
				live = append(live, n)
			}
		}
		_ = saveCardsCache(app, live)
		return nil
	case *oas.ListCardsBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListCardsUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListCardsInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

type sealedCard struct {
	nonce      []byte
	ciphertext []byte
	sum        [sha256.Size]byte
}

func sealCard(app *App, plain CardPlain) (sealedCard, error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return sealedCard{}, fmt.Errorf("login first")
	}
	raw, err := json.Marshal(plain)
	if err != nil {
		return sealedCard{}, err
	}
	nonce, ct, err := vault.Seal(key, raw)
	if err != nil {
		return sealedCard{}, err
	}
	return sealedCard{nonce: nonce, ciphertext: ct, sum: sha256.Sum256(ct)}, nil
}

func openCard(app *App, n oas.Card) (CardPlain, string, error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return CardPlain{}, "", fmt.Errorf("login first")
	}
	raw, err := vault.Open(key, append(n.Nonce, n.Ciphertext...))
	if err != nil {
		return CardPlain{}, "", err
	}
	var plain CardPlain
	if err := json.Unmarshal(raw, &plain); err != nil {
		return CardPlain{}, "", err
	}
	meta := ""
	if n.Meta.IsSet() {
		meta = n.Meta.Value
	}
	return plain, meta, nil
}

func fetchCard(ctx context.Context, app *App, id uuid.UUID) (oas.Card, error) {
	res, err := app.Client.CardGet(ctx, oas.CardGetParams{ID: id})
	if err != nil {
		return oas.Card{}, err
	}
	n, ok := res.(*oas.Card)
	if !ok {
		return oas.Card{}, cardMsg(res)
	}
	return *n, nil
}

func handleCardRes(app *App, res any, w io.Writer) error {
	if n, ok := res.(*oas.Card); ok {
		if err := printCard(app, w, *n); err != nil {
			return err
		}
		syncCardCache(app, *n)
		return nil
	}
	return cardMsg(res)
}

func toCachedCard(app *App, n oas.Card) (cachedCard, bool) {
	if n.DeletedAt.IsSet() {
		return cachedCard{}, false
	}
	plain, meta, err := openCard(app, n)
	if err != nil {
		return cachedCard{}, false
	}
	return cachedCard{
		ID: n.ID, Version: n.Version,
		Number: plain.Number, Holder: plain.Holder, Expiry: plain.Expiry, CVV: plain.CVV,
		Meta: meta,
	}, true
}

func syncCardCache(app *App, n oas.Card) {
	if n.DeletedAt.IsSet() {
		removeCardsCache(app, n.ID)
		return
	}
	if cached, ok := toCachedCard(app, n); ok {
		upsertCardsCache(app, cached)
	}
}

func writeCachedCards(app *App, w io.Writer) error {
	cards, err := loadCardsCache(app)
	if err != nil {
		return fmt.Errorf("offline cards: %w", err)
	}
	for _, n := range cards {
		if _, err := fmt.Fprintf(w, "%s v%d\n%s %s %s %s\n%s\n", n.ID, n.Version, maskCardNumber(n.Number), n.Holder, n.Expiry, n.CVV, n.Meta); err != nil {
			return err
		}
	}
	return nil
}

func writeCachedCard(app *App, id uuid.UUID, w io.Writer) error {
	cards, err := loadCardsCache(app)
	if err != nil {
		return fmt.Errorf("offline cards: %w", err)
	}
	for _, n := range cards {
		if n.ID == id {
			_, err := fmt.Fprintf(w, "%s v%d\n%s %s %s %s\n%s\n", n.ID, n.Version, maskCardNumber(n.Number), n.Holder, n.Expiry, n.CVV, n.Meta)
			return err
		}
	}
	return fmt.Errorf("card not found in offline cache")
}

func printCard(app *App, w io.Writer, n oas.Card) error {
	plain, meta, err := openCard(app, n)
	if err != nil {
		return err
	}
	deleted := ""
	if n.DeletedAt.IsSet() {
		deleted = " deleted"
	}
	_, err = fmt.Fprintf(w, "%s v%d%s\n%s %s %s %s\n%s\n", n.ID, n.Version, deleted, maskCardNumber(plain.Number), plain.Holder, plain.Expiry, plain.CVV, meta)
	return err
}

func maskCardNumber(number string) string {
	n := strings.ReplaceAll(number, " ", "")
	if len(n) <= 4 {
		return n
	}
	return "****" + n[len(n)-4:]
}

func cardMsg(res any) error {
	switch v := res.(type) {
	case *oas.CardCreateBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardCreateConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardCreateUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardCreateInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardUpdateBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardUpdateConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardUpdateUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardUpdateNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardUpdateInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardDeleteUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardDeleteNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardDeleteInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardGetUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardGetNotFound:
		return fmt.Errorf("%s", v.Msg)
	case *oas.CardGetInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}
