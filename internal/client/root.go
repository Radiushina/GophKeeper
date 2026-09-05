package client

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Radiushina/GophKeeper/internal/domains/buildinfo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Run(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return repl(ctx, app, os.Stdin, os.Stdout)
	}
	err := execCommand(ctx, app, args)
	if err != nil {
		app.logError("command failed", err, zap.String("command", args[0]))
	}
	return err
}

func repl(ctx context.Context, app *App, in io.Reader, out io.Writer) error {
	app.logInfo("GophKeeper. Commands: register, login, note-add, note-update, note-delete, note-get, note-list, tui, version, exit")
	sc := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
		flushWriter(out)
		if !sc.Scan() {
			err := sc.Err()
			app.logError("repl", err)
			return err
		}
		line := strings.TrimSpace(strings.ToValidUTF8(sc.Text(), ""))
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		if isQuit(args[0]) {
			return nil
		}
		if err := execCommand(ctx, app, args); err != nil {
			app.logError("command failed", err, zap.String("command", args[0]))
		}
	}
}

func execCommand(ctx context.Context, app *App, args []string) error {
	switch args[0] {
	case "register":
		login, password, err := parseAuthFlags(app, "register", args[1:])
		if err != nil {
			return err
		}
		return Register(ctx, app, login, password)
	case "login":
		login, password, err := parseAuthFlags(app, "login", args[1:])
		if err != nil {
			return err
		}
		return Login(ctx, app, login, password)
	case "note-add":
		text, meta, err := parseNoteAddFlags(app, args[1:])
		if err != nil {
			return err
		}
		return NoteAdd(ctx, app, text, meta)
	case "note-update":
		upd, err := parseNoteUpdateFlags(app, args[1:])
		if err != nil {
			return err
		}
		return NoteUpdate(ctx, app, upd.id, upd.version, upd.text, upd.meta)
	case "note-delete":
		id, err := parseNoteIDFlags(app, "note-delete", args[1:])
		if err != nil {
			return err
		}
		return NoteDelete(ctx, app, id)
	case "note-get":
		id, err := parseNoteIDFlags(app, "note-get", args[1:])
		if err != nil {
			return err
		}
		return NoteGet(ctx, app, id)
	case "note-list":
		since, err := parseNoteListFlags(app, args[1:])
		if err != nil {
			return err
		}
		return NoteList(ctx, app, since)
	case "tui":
		return RunTUI(ctx, app)
	case "version":
		buildinfo.Print()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseAuthFlags(app *App, name string, args []string) (login, password string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(app.logWriter())
	fs.StringVar(&login, "login", "", "login")
	fs.StringVar(&password, "password", "", "password")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if login == "" || password == "" {
		return "", "", fmt.Errorf("login and password are required")
	}
	return login, password, nil
}

func parseNoteAddFlags(app *App, args []string) (text, meta string, err error) {
	fs := flag.NewFlagSet("note-add", flag.ContinueOnError)
	fs.SetOutput(app.logWriter())
	fs.StringVar(&text, "text", "", "note text")
	fs.StringVar(&meta, "meta", "", "optional metadata")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if text == "" && meta == "" {
		return "", "", fmt.Errorf("text or meta is required")
	}
	return text, meta, nil
}

type noteUpdateArgs struct {
	id      uuid.UUID
	version int64
	text    string
	meta    string
}

func parseNoteUpdateFlags(app *App, args []string) (noteUpdateArgs, error) {
	fs := flag.NewFlagSet("note-update", flag.ContinueOnError)
	fs.SetOutput(app.logWriter())
	var (
		rawID   string
		version int64
		text    string
		meta    string
	)
	fs.StringVar(&rawID, "id", "", "note id")
	fs.Int64Var(&version, "version", 0, "current server version; 0 means fetch first")
	fs.StringVar(&text, "text", "", "note text")
	fs.StringVar(&meta, "meta", "", "optional metadata")
	if err := fs.Parse(args); err != nil {
		return noteUpdateArgs{}, err
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return noteUpdateArgs{}, fmt.Errorf("id is required")
	}
	if text == "" && meta == "" {
		return noteUpdateArgs{}, fmt.Errorf("text or meta is required")
	}
	return noteUpdateArgs{id: id, version: version, text: text, meta: meta}, nil
}

func parseNoteIDFlags(app *App, name string, args []string) (uuid.UUID, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(app.logWriter())
	var rawID string
	fs.StringVar(&rawID, "id", "", "note id")
	if err := fs.Parse(args); err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("id is required")
	}
	return id, nil
}

func parseNoteListFlags(app *App, args []string) (*time.Time, error) {
	fs := flag.NewFlagSet("note-list", flag.ContinueOnError)
	fs.SetOutput(app.logWriter())
	var raw string
	fs.StringVar(&raw, "since", "", "RFC3339 sync cursor")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("since must be RFC3339")
	}
	return &t, nil
}

func isQuit(cmd string) bool {
	cmd = strings.ToLower(strings.TrimSpace(strings.ToValidUTF8(cmd, "")))
	return cmd == "exit" || cmd == "quit" || strings.HasSuffix(cmd, "exit") || strings.HasSuffix(cmd, "quit")
}

func flushWriter(w io.Writer) {
	type flusher interface{ Flush() error }
	if f, ok := w.(flusher); ok {
		_ = f.Flush()
		return
	}
	if f, ok := w.(*os.File); ok {
		_ = f.Sync()
	}
}
