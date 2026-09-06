package client

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

// NotePlain is the decrypted payload stored inside ciphertext.
type NotePlain struct {
	Text string `json:"text"`
	Meta string `json:"meta"`
}

// NoteAdd encrypts a note and creates it on the server.
func NoteAdd(ctx context.Context, app *App, text, meta string) error {
	blob, err := sealNote(app, NotePlain{Text: text, Meta: meta})
	if err != nil {
		return err
	}
	res, err := app.Client.NoteCreate(ctx, &oas.CreateNote{
		ID:               uuid.New(),
		Version:          note.CreateVersion,
		Nonce:            blob.nonce,
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, os.Stdout)
}

// NoteUpdate encrypts a replacement blob and writes it with optimistic locking.
func NoteUpdate(ctx context.Context, app *App, id uuid.UUID, version int64, text, meta string) error {
	if version == 0 {
		current, err := fetchNote(ctx, app, id)
		if err != nil {
			return err
		}
		version = current.Version
	}
	blob, err := sealNote(app, NotePlain{Text: text, Meta: meta})
	if err != nil {
		return err
	}
	res, err := app.Client.NoteUpdate(ctx, &oas.UpdateNote{
		Version:          version,
		Nonce:            blob.nonce,
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	}, oas.NoteUpdateParams{ID: id})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, os.Stdout)
}

// NoteDelete tombstones a note on the server.
func NoteDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.NoteDelete(ctx, oas.NoteDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, os.Stdout)
}

// NoteGet fetches and decrypts one note.
func NoteGet(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.NoteGet(ctx, oas.NoteGetParams{ID: id})
	if err != nil {
		if isOffline(err) {
			return writeCachedNote(app, id, os.Stdout)
		}
		return err
	}
	return handleNoteRes(app, res, os.Stdout)
}

// NoteList prints decrypted notes. since is the optional sync cursor.
func NoteList(ctx context.Context, app *App, since *time.Time) error {
	return writeNoteList(ctx, app, since, os.Stdout)
}

func writeNoteList(ctx context.Context, app *App, since *time.Time, w io.Writer) error {
	params := oas.ListNotesParams{}
	if since != nil {
		params.Since = oas.NewOptDateTime(*since)
	}
	res, err := app.Client.ListNotes(ctx, params)
	if err != nil {
		if isOffline(err) {
			return writeCachedNotes(app, w)
		}
		return err
	}
	switch v := res.(type) {
	case *oas.NoteListRes:
		live := make([]cachedNote, 0, len(v.Items))
		for i := range v.Items {
			if err := printNote(app, w, v.Items[i]); err != nil {
				return err
			}
			if n, ok := toCachedNote(app, v.Items[i]); ok {
				live = append(live, n)
			}
		}
		_ = saveNotesCache(app, live)
		return nil
	case *oas.ListNotesBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListNotesUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.ListNotesInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

type sealedNote struct {
	nonce      []byte
	ciphertext []byte
	sum        [sha256.Size]byte
}

func sealNote(app *App, plain NotePlain) (sealedNote, error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return sealedNote{}, fmt.Errorf("login first")
	}
	raw, err := json.Marshal(plain)
	if err != nil {
		return sealedNote{}, err
	}
	nonce, ct, err := vault.Seal(key, raw)
	if err != nil {
		return sealedNote{}, err
	}
	return sealedNote{nonce: nonce, ciphertext: ct, sum: sha256.Sum256(ct)}, nil
}

func openNote(app *App, n oas.Note) (NotePlain, error) {
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return NotePlain{}, fmt.Errorf("login first")
	}
	raw, err := vault.Open(key, append(n.Nonce, n.Ciphertext...))
	if err != nil {
		return NotePlain{}, err
	}
	var plain NotePlain
	if err := json.Unmarshal(raw, &plain); err != nil {
		return NotePlain{}, err
	}
	return plain, nil
}

func fetchNote(ctx context.Context, app *App, id uuid.UUID) (oas.Note, error) {
	res, err := app.Client.NoteGet(ctx, oas.NoteGetParams{ID: id})
	if err != nil {
		return oas.Note{}, err
	}
	n, ok := res.(*oas.Note)
	if !ok {
		return oas.Note{}, noteMsg(res)
	}
	return *n, nil
}

func handleNoteRes(app *App, res any, w io.Writer) error {
	if n, ok := res.(*oas.Note); ok {
		if err := printNote(app, w, *n); err != nil {
			return err
		}
		syncNoteCache(app, *n)
		return nil
	}
	return noteMsg(res)
}

func toCachedNote(app *App, n oas.Note) (cachedNote, bool) {
	if n.DeletedAt.IsSet() {
		return cachedNote{}, false
	}
	plain, err := openNote(app, n)
	if err != nil {
		return cachedNote{}, false
	}
	return cachedNote{ID: n.ID, Version: n.Version, Text: plain.Text, Meta: plain.Meta}, true
}

func syncNoteCache(app *App, n oas.Note) {
	if n.DeletedAt.IsSet() {
		removeNotesCache(app, n.ID)
		return
	}
	if cached, ok := toCachedNote(app, n); ok {
		upsertNotesCache(app, cached)
	}
}

func writeCachedNotes(app *App, w io.Writer) error {
	notes, err := loadNotesCache(app)
	if err != nil {
		return fmt.Errorf("offline notes: %w", err)
	}
	for _, n := range notes {
		if _, err := fmt.Fprintf(w, "%s v%d\n%s\n%s\n", n.ID, n.Version, n.Text, n.Meta); err != nil {
			return err
		}
	}
	return nil
}

func writeCachedNote(app *App, id uuid.UUID, w io.Writer) error {
	notes, err := loadNotesCache(app)
	if err != nil {
		return fmt.Errorf("offline notes: %w", err)
	}
	for _, n := range notes {
		if n.ID == id {
			_, err := fmt.Fprintf(w, "%s v%d\n%s\n%s\n", n.ID, n.Version, n.Text, n.Meta)
			return err
		}
	}
	return fmt.Errorf("note not found in offline cache")
}

func printNote(app *App, w io.Writer, n oas.Note) error {
	plain, err := openNote(app, n)
	if err != nil {
		return err
	}
	deleted := ""
	if n.DeletedAt.IsSet() {
		deleted = " deleted"
	}
	_, err = fmt.Fprintf(w, "%s v%d%s\n%s\n%s\n", n.ID, n.Version, deleted, plain.Text, plain.Meta)
	return err
}

func noteMsg(res any) error {
	switch v := res.(type) {
	case *oas.NoteCreateBadRequest, *oas.NoteUpdateBadRequest:
		return fmt.Errorf("%s", msgOf(v))
	case *oas.NoteCreateConflict, *oas.NoteUpdateConflict:
		return fmt.Errorf("%s", msgOf(v))
	case *oas.NoteCreateUnauthorized, *oas.NoteUpdateUnauthorized, *oas.NoteDeleteUnauthorized, *oas.NoteGetUnauthorized:
		return fmt.Errorf("%s", msgOf(v))
	case *oas.NoteUpdateNotFound, *oas.NoteDeleteNotFound, *oas.NoteGetNotFound:
		return fmt.Errorf("%s", msgOf(v))
	case *oas.NoteCreateInternalServerError, *oas.NoteUpdateInternalServerError, *oas.NoteDeleteInternalServerError, *oas.NoteGetInternalServerError:
		return fmt.Errorf("%s", msgOf(v))
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

func msgOf(v any) string {
	switch t := v.(type) {
	case *oas.NoteCreateBadRequest:
		return t.Msg
	case *oas.NoteCreateConflict:
		return t.Msg
	case *oas.NoteCreateUnauthorized:
		return t.Msg
	case *oas.NoteCreateInternalServerError:
		return t.Msg
	case *oas.NoteUpdateBadRequest:
		return t.Msg
	case *oas.NoteUpdateConflict:
		return t.Msg
	case *oas.NoteUpdateUnauthorized:
		return t.Msg
	case *oas.NoteUpdateNotFound:
		return t.Msg
	case *oas.NoteUpdateInternalServerError:
		return t.Msg
	case *oas.NoteDeleteUnauthorized:
		return t.Msg
	case *oas.NoteDeleteNotFound:
		return t.Msg
	case *oas.NoteDeleteInternalServerError:
		return t.Msg
	case *oas.NoteGetUnauthorized:
		return t.Msg
	case *oas.NoteGetNotFound:
		return t.Msg
	case *oas.NoteGetInternalServerError:
		return t.Msg
	default:
		return "error"
	}
}
