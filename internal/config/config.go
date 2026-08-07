// Package config loads and persists orgen settings.
//
// Settings live in config.yaml under $ORGEN_CONFIG_DIR, or the
// user's XDG config directory under "orgen" when unset. Every key is
// readable from an ORGEN_-prefixed environment variable (dots become
// underscores: provider.sort -> ORGEN_PROVIDER_SORT), and environment
// values rank above file values. The package never prints; failures
// surface as returned errors.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Well-known keys. Provider routing settings nest under "provider."
// (e.g. "provider.sort").
const (
	KeyModel             = "model"
	KeyAspectRatio       = "aspect_ratio"
	KeyOutputFormat      = "output_format"
	KeyOutputCompression = "output_compression"
	KeyProvider          = "provider"
)

// EnvConfigDir overrides the directory holding config.yaml.
const EnvConfigDir = "ORGEN_CONFIG_DIR"

const (
	envPrefix = "orgen"
	appDir    = "orgen"
	fileName  = "config.yaml"
	dirPerm   = 0o700
	filePerm  = 0o600
)

// Config reads and persists user settings. Implementations must not
// print; all failures surface as returned errors.
type Config interface {
	// Model returns the default image model, or "".
	Model() string
	// AspectRatio returns the default aspect ratio, or "".
	AspectRatio() string
	// OutputFormat returns the default output format, or "".
	OutputFormat() string
	// OutputCompression returns the default compression level and
	// whether one is configured.
	OutputCompression() (int, bool)
	// Provider returns file-backed provider routing settings; individual
	// environment overrides are readable via Get("provider.<setting>").
	Provider() map[string]any
	// AllKeys returns every addressable key — the well-known scalars plus
	// anything present in the file — sorted.
	AllKeys() []string
	// Get returns the effective value for a scalar key, "" when unset.
	Get(key string) (string, error)
	// Set stages a value for a key; Save persists it.
	Set(key, value string) error
	// Save writes file-backed and staged values to Path. Environment
	// overrides are never written.
	Save() error
	// Path is the config file location.
	Path() string
}

// scalarKeys are the top-level keys bound to ORGEN_* environment
// variables at load time; nested provider.* keys are served by
// viper's AutomaticEnv instead.
func scalarKeys() []string {
	return []string{KeyModel, KeyAspectRatio, KeyOutputFormat, KeyOutputCompression}
}

// filePath resolves the config file location: $ORGEN_CONFIG_DIR when
// set, else <user-config-dir>/orgen.
func filePath() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return filepath.Join(dir, fileName), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user config dir: %w", err)
	}
	return filepath.Join(base, appDir, fileName), nil
}

// store is the viper-backed Config implementation.
type store struct {
	// main layers environment variables over the config file and serves
	// all reads. file holds only file-backed and explicitly Set values —
	// viper's WriteConfig persists AllSettings(), which on an env-aware
	// instance would write environment overrides to disk.
	main *viper.Viper
	file *viper.Viper
}

var _ Config = (*store)(nil)

// New loads the configuration. A missing config file is not an error.
//
//nolint:ireturn // constructor deliberately returns the mockable interface
func New() (Config, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	main := viper.New()
	main.SetEnvPrefix(envPrefix)
	main.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	main.AutomaticEnv()
	for _, key := range scalarKeys() {
		// BindEnv (not SetDefault) registers the key so IsSet and
		// AllKeys see it without a default value poisoning IsSet.
		if bindErr := main.BindEnv(key); bindErr != nil {
			return nil, fmt.Errorf("binding environment for %s: %w", key, bindErr)
		}
	}
	file := viper.New()
	for _, v := range []*viper.Viper{main, file} {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		v.SetConfigPermissions(filePerm)
		if readErr := readConfig(v, path); readErr != nil {
			return nil, readErr
		}
	}
	return &store{main: main, file: file}, nil
}

func readConfig(v *viper.Viper, path string) error {
	err := v.ReadInConfig()
	var notFound viper.ConfigFileNotFoundError
	if err == nil || errors.Is(err, fs.ErrNotExist) || errors.As(err, &notFound) {
		return nil
	}
	return fmt.Errorf("reading config %s: %w", path, err)
}

func (s *store) Model() string        { return s.main.GetString(KeyModel) }
func (s *store) AspectRatio() string  { return s.main.GetString(KeyAspectRatio) }
func (s *store) OutputFormat() string { return s.main.GetString(KeyOutputFormat) }

func (s *store) OutputCompression() (int, bool) {
	if !s.main.IsSet(KeyOutputCompression) {
		return 0, false
	}
	return s.main.GetInt(KeyOutputCompression), true
}

func (s *store) Provider() map[string]any { return s.main.GetStringMap(KeyProvider) }

func (s *store) AllKeys() []string {
	keys := s.main.AllKeys()
	slices.Sort(keys)
	return keys
}

func (s *store) Get(key string) (string, error) {
	if err := s.validateScalarKey(key); err != nil {
		return "", err
	}
	return s.main.GetString(key), nil
}

func (s *store) Set(key, value string) error {
	if err := s.validateScalarKey(key); err != nil {
		return err
	}
	var val any = value
	if key == KeyOutputCompression {
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("value for %s must be an integer, got %q", key, value)
		}
		val = n
	}
	s.main.Set(key, val)
	s.file.Set(key, val)
	return nil
}

func (s *store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path()), dirPerm); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := s.file.WriteConfig(); err != nil {
		return fmt.Errorf("writing config %s: %w", s.Path(), err)
	}
	return nil
}

func (s *store) Path() string { return s.main.ConfigFileUsed() }

// validateScalarKey accepts keys viper knows about (bound scalars plus
// anything loaded or set) and not-yet-set provider.<setting> paths;
// the bare provider map is not scalar-addressable.
func (s *store) validateScalarKey(key string) error {
	if key == KeyProvider {
		return fmt.Errorf("%q holds a settings map: address %s.<setting> instead", KeyProvider, KeyProvider)
	}
	if slices.Contains(s.main.AllKeys(), key) || strings.HasPrefix(key, KeyProvider+".") {
		return nil
	}
	return fmt.Errorf("unknown config key %q (known keys: %s)", key, strings.Join(s.AllKeys(), ", "))
}
