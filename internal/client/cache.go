package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

type sessionCache struct {
	Login          string `json:"login"`
	KdfSalt        []byte `json:"kdf_salt"`
	ProtectedKey   []byte `json:"protected_key"`
	KeyHash        []byte `json:"key_hash"`
	KdfMemory      uint32 `json:"kdf_memory"`
	KdfIterations  uint32 `json:"kdf_iterations"`
	KdfParallelism uint8  `json:"kdf_parallelism"`
	KdfVersion     uint8  `json:"kdf_version"`
}

type cachedNote struct {
	ID      uuid.UUID `json:"id"`
	Version int64     `json:"version"`
	Text    string    `json:"text"`
	Meta    string    `json:"meta"`
}

type cachedCard struct {
	ID      uuid.UUID `json:"id"`
	Version int64     `json:"version"`
	Number  string    `json:"number"`
	Holder  string    `json:"holder"`
	Expiry  string    `json:"expiry"`
	CVV     string    `json:"cvv"`
	Meta    string    `json:"meta"`
}

func isOffline(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "network is unreachable") ||
		strings.Contains(s, "connection reset")
}

func cacheRoot(app *App) string {
	if app != nil && app.CacheDir != "" {
		return app.CacheDir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "gophkeeper")
	}
	return filepath.Join(dir, "gophkeeper")
}

func userCacheDir(app *App, login string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(login)))
	return filepath.Join(cacheRoot(app), hex.EncodeToString(sum[:8]))
}

func cacheEnabled(app *App) bool {
	return app != nil && app.CacheDir != ""
}

func saveSession(app *App, sess sessionCache) error {
	if !cacheEnabled(app) {
		return nil
	}
	if sess.Login == "" {
		return fmt.Errorf("login is required")
	}
	dir := userCacheDir(app, sess.Login)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), raw, 0o600)
}

func loadSession(app *App, login string) (sessionCache, error) {
	raw, err := os.ReadFile(filepath.Join(userCacheDir(app, login), "session.json"))
	if err != nil {
		return sessionCache{}, err
	}
	var sess sessionCache
	if err := json.Unmarshal(raw, &sess); err != nil {
		return sessionCache{}, err
	}
	return sess, nil
}

func saveNotesCache(app *App, notes []cachedNote) error {
	if !cacheEnabled(app) {
		return nil
	}
	login := app.User()
	if login == "" {
		return fmt.Errorf("login first")
	}
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return fmt.Errorf("login first")
	}
	raw, err := json.Marshal(notes)
	if err != nil {
		return err
	}
	nonce, ct, err := vault.Seal(key, raw)
	if err != nil {
		return err
	}
	dir := userCacheDir(app, login)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "notes.bin"), append(nonce, ct...), 0o600)
}

func loadNotesCache(app *App) ([]cachedNote, error) {
	login := app.User()
	if login == "" {
		return nil, fmt.Errorf("login first")
	}
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return nil, fmt.Errorf("login first")
	}
	raw, err := os.ReadFile(filepath.Join(userCacheDir(app, login), "notes.bin"))
	if err != nil {
		return nil, err
	}
	plain, err := vault.Open(key, raw)
	if err != nil {
		return nil, err
	}
	var notes []cachedNote
	if err := json.Unmarshal(plain, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

func upsertNotesCache(app *App, note cachedNote) {
	items, err := loadNotesCache(app)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		items = nil
	}
	found := false
	for i := range items {
		if items[i].ID == note.ID {
			items[i] = note
			found = true
			break
		}
	}
	if !found {
		items = append(items, note)
	}
	_ = saveNotesCache(app, items)
}

func removeNotesCache(app *App, id uuid.UUID) {
	items, err := loadNotesCache(app)
	if err != nil {
		return
	}
	out := items[:0]
	for _, n := range items {
		if n.ID != id {
			out = append(out, n)
		}
	}
	_ = saveNotesCache(app, out)
}

func saveCardsCache(app *App, cards []cachedCard) error {
	if !cacheEnabled(app) {
		return nil
	}
	login := app.User()
	if login == "" {
		return fmt.Errorf("login first")
	}
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return fmt.Errorf("login first")
	}
	raw, err := json.Marshal(cards)
	if err != nil {
		return err
	}
	nonce, ct, err := vault.Seal(key, raw)
	if err != nil {
		return err
	}
	dir := userCacheDir(app, login)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cards.bin"), append(nonce, ct...), 0o600)
}

func loadCardsCache(app *App) ([]cachedCard, error) {
	login := app.User()
	if login == "" {
		return nil, fmt.Errorf("login first")
	}
	key := app.VaultKey()
	if len(key) != vault.VKSize {
		return nil, fmt.Errorf("login first")
	}
	raw, err := os.ReadFile(filepath.Join(userCacheDir(app, login), "cards.bin"))
	if err != nil {
		return nil, err
	}
	plain, err := vault.Open(key, raw)
	if err != nil {
		return nil, err
	}
	var cards []cachedCard
	if err := json.Unmarshal(plain, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func upsertCardsCache(app *App, card cachedCard) {
	items, err := loadCardsCache(app)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		items = nil
	}
	found := false
	for i := range items {
		if items[i].ID == card.ID {
			items[i] = card
			found = true
			break
		}
	}
	if !found {
		items = append(items, card)
	}
	_ = saveCardsCache(app, items)
}

func removeCardsCache(app *App, id uuid.UUID) {
	items, err := loadCardsCache(app)
	if err != nil {
		return
	}
	out := items[:0]
	for _, n := range items {
		if n.ID != id {
			out = append(out, n)
		}
	}
	_ = saveCardsCache(app, out)
}

func loginOffline(app *App, login, password string) error {
	sess, err := loadSession(app, login)
	if err != nil {
		return fmt.Errorf("offline login: %w", err)
	}
	vk, err := vault.Unwrap(password, vault.Material{
		Salt:         sess.KdfSalt,
		ProtectedKey: sess.ProtectedKey,
		KeyHash:      sess.KeyHash,
	}, vault.Params{
		Memory:      sess.KdfMemory,
		Iterations:  sess.KdfIterations,
		Parallelism: sess.KdfParallelism,
		Version:     sess.KdfVersion,
	})
	if err != nil {
		return fmt.Errorf("offline login: %w", err)
	}
	app.SetUser(login)
	app.SetToken("")
	app.SetVaultKey(vk)
	return nil
}
