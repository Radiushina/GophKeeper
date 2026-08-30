package client

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Radiushina/GophKeeper/internal/domains/buildinfo"
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
	app.logInfo("GophKeeper. Commands: register, login, version, exit")
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
