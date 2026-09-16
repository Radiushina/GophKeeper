package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Radiushina/GophKeeper/internal/domains/buildinfo"
)

const (
	defaultServer     = "http://localhost:8080"
	defaultConfigPath = ""
	defaultLogLevel   = "info"
)

type Flags struct {
	server     string
	configPath string
	logLevel   string
	version    bool
	tui        bool
	args       []string
}

func NewFlags() *Flags {
	return &Flags{
		server:     defaultServer,
		configPath: defaultConfigPath,
		logLevel:   defaultLogLevel,
	}
}

func newFlags() (*Flags, error) {
	f := NewFlags()
	if _, err := f.parse(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		return nil, err
	}
	if f.version {
		buildinfo.Print()
		os.Exit(0)
	}
	return f, nil
}

// parse загружает конфиг в порядке: defaults → flags → YAML (только незаданные) → ENV.
func (r *Flags) parse() (exitCode int, err error) {
	*r = *NewFlags()

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&r.configPath, "c", defaultConfigPath, "path to YAML config file")
	fs.StringVar(&r.configPath, "config", defaultConfigPath, "path to YAML config file")
	fs.StringVar(&r.server, "a", defaultServer, "remote API URL")
	fs.StringVar(&r.server, "server", defaultServer, "remote API URL")
	fs.StringVar(&r.logLevel, "log-level", defaultLogLevel, "log level")
	fs.BoolVar(&r.version, "version", false, "print build version and date")
	fs.BoolVar(&r.tui, "tui", false, "open terminal UI (REPL stays the default)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, flag.ErrHelp
		}
		return 1, err
	}

	visited := visitedFlags(fs)
	configPath := resolveConfigPath(r.configPath, visited)
	if configPath != "" {
		fileCfg, err := loadClientFile(configPath)
		if err != nil {
			return 1, err
		}
		r.applyFileIfUnset(fileCfg, visited)
		if !visited["c"] && !visited["config"] {
			r.configPath = configPath
		}
	}

	r.applyEnv()
	r.args = fs.Args()
	return 0, nil
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})
	return seen
}

func resolveConfigPath(flagPath string, visited map[string]bool) string {
	if visited["c"] || visited["config"] {
		return strings.TrimSpace(flagPath)
	}
	if p := strings.TrimSpace(os.Getenv("GOPHKEEPER_CONFIG")); p != "" {
		return p
	}
	const fallback = "config/content.yml"
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return strings.TrimSpace(flagPath)
}

type clientFileConfig struct {
	Client *struct {
		HTTP *struct {
			Address *string `yaml:"address"`
		} `yaml:"http"`
	} `yaml:"client"`
	Log *struct {
		Level *string `yaml:"level"`
	} `yaml:"log"`
}

func loadClientFile(path string) (clientFileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return clientFileConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg clientFileConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return clientFileConfig{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func (r *Flags) applyFileIfUnset(cfg clientFileConfig, visited map[string]bool) {
	if !visited["a"] && !visited["server"] {
		if cfg.Client != nil && cfg.Client.HTTP != nil && cfg.Client.HTTP.Address != nil {
			if addr := strings.TrimSpace(*cfg.Client.HTTP.Address); addr != "" {
				r.server = addr
			}
		}
	}
	if !visited["log-level"] && cfg.Log != nil && cfg.Log.Level != nil {
		if lvl := strings.TrimSpace(*cfg.Log.Level); lvl != "" {
			r.logLevel = lvl
		}
	}
}

func (r *Flags) applyEnv() {
	if v := lookupEnv("CLIENT_HTTP_ADDRESS"); v != "" {
		r.server = v
	}
	if v := lookupEnv("LOG_LEVEL"); v != "" {
		r.logLevel = v
	}
}

func lookupEnv(name string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("GOPHKEEPER_" + name))
}

func (r *Flags) serverBaseURL() string {
	s := strings.TrimSpace(r.server)
	if s == "" {
		s = defaultServer
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	s = strings.TrimPrefix(s, "/")
	return "http://" + s
}
