package buildinfo

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Values are overwritten at link time via -ldflags.
var (
	Version = "0.1.0"
	Date    = "N/A"
)

func Print() {
	Fprint(os.Stdout)
}

func Fprint(w io.Writer) {
	version, date := resolved()
	_, _ = fmt.Fprintf(w, "Build version: %s\n", version)
	_, _ = fmt.Fprintf(w, "Build date: %s\n", date)
}

func resolved() (version, date string) {
	version, date = na(Version), na(Date)
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.time" && s.Value != "" && date == "N/A" {
				date = s.Value
			}
		}
	}
	if date == "N/A" {
		if t := gitDate(); t != "" {
			date = t
		}
	}
	return version, date
}

var gitOnce sync.Once
var gitTime string

func gitDate() string {
	gitOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "log", "-1", "--format=%cI").Output()
		if err != nil {
			return
		}
		gitTime = strings.TrimSpace(string(out))
	})
	return gitTime
}

func na(s string) string {
	if strings.TrimSpace(s) == "" {
		return "N/A"
	}
	return s
}
