package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

const (
	defaultConfigPath = "config/server/content.yml"
	structTagDelim    = "."
	envPrefix         = "GOPHKEEPER_"
)

type Loader struct {
	k          *koanf.Koanf
	configPath string
	flags      *pflag.FlagSet
}

func NewLoader(configPath string) *Loader {
	if configPath == "" {
		configPath = defaultConfigPath
	}
	return &Loader{
		k:          koanf.New(structTagDelim),
		configPath: configPath,
		flags:      pflag.NewFlagSet("config", pflag.ContinueOnError),
	}
}

func (l *Loader) Load() (*Config, error) {
	if err := l.loadDefaults(); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	l.defineFlags()
	if err := l.flags.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	if err := l.loadYAML(); err != nil {
		return nil, fmt.Errorf("failed to load YAML config: %w", err)
	}

	if err := l.loadEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	if err := l.loadFlags(); err != nil {
		return nil, fmt.Errorf("failed to apply CLI flags: %w", err)
	}

	var cfg Config
	if err := l.k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (l *Loader) loadDefaults() error {
	defaults := DefaultConfig()
	if err := l.k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return fmt.Errorf("loadDefaults: %w", err)
	}
	return nil
}

func (l *Loader) loadYAML() error {
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		log.Printf("yaml config not found at %s, using defaults+env+flags", l.configPath)
		return nil
	}
	if err := l.k.Load(file.Provider(l.configPath), yaml.Parser()); err != nil {
		return fmt.Errorf("loadYAML: %w", err)
	}
	log.Printf("yaml config loaded from %s", l.configPath)
	return nil
}

func (l *Loader) loadEnv() error {
	return l.setFromEnv(reflect.TypeOf(Config{}), "")
}

func (l *Loader) loadFlags() error {
	flagMapping := map[string]string{
		"http-address": "server.http.address",
		"dsn":          "database.dsn",
		"jwt-secret":   "auth.jwt_secret",
		"log-level":    "log.level",
	}

	var visitErr error
	l.flags.Visit(func(f *pflag.Flag) {
		if visitErr != nil {
			return
		}
		path, ok := flagMapping[f.Name]
		if !ok {
			return
		}
		if err := l.k.Set(path, f.Value.String()); err != nil {
			visitErr = fmt.Errorf("loadFlags: set %s: %w", path, err)
		}
	})
	return visitErr
}

func (l *Loader) setFromEnv(t reflect.Type, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("koanf")
		if tag == "" || tag == "-" {
			continue
		}
		path := tag
		if prefix != "" {
			path = prefix + structTagDelim + tag
		}

		if f.Type.Kind() == reflect.Struct {
			if err := l.setFromEnv(f.Type, path); err != nil {
				return err
			}
			continue
		}

		envName := envPrefix + strings.ToUpper(strings.ReplaceAll(path, structTagDelim, "_"))
		raw, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}

		var value any = raw
		if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String {
			value = strings.Split(raw, ",")
		}
		if err := l.k.Set(path, value); err != nil {
			return fmt.Errorf("loadEnv: set %s: %w", path, err)
		}
	}
	return nil
}

func (l *Loader) defineFlags() {
	l.flags.StringVar(&l.configPath, "config", l.configPath, "path to YAML config")
	l.flags.String("http-address", "", "HTTP listen address")
	l.flags.String("dsn", "", "Postgres DSN")
	l.flags.String("jwt-secret", "", "HMAC secret for JWT")
	l.flags.String("log-level", "", "zap log level")
}
