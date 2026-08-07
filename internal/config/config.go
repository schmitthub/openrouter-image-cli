// Package config loads and persists orimage settings.
//
// Settings live in config.yaml under $ORIMAGE_CONFIG_DIR, or the
// user's XDG config directory under "orimage" when unset. Every key is
// readable from an ORIMAGE_-prefixed environment variable (dots become
// underscores: provider.sort -> ORIMAGE_PROVIDER_SORT), and environment
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

// Keys addressable in the config file, through ORIMAGE_* environment
// variables, and by Get/Set. Provider settings nest under "provider."
// (e.g. "provider.sort").
const (
	KeyModel             = "model"
	KeyAspectRatio       = "aspect_ratio"
	KeyOutputFormat      = "output_format"
	KeyOutputCompression = "output_compression"
	KeyProvider          = "provider"
)

// EnvConfigDir overrides the directory holding config.yaml.
const EnvConfigDir = "ORIMAGE_CONFIG_DIR"

const (
	envPrefix = "orimage"
	appDir    = "orimage"
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
	// Get returns the effective value for a known scalar key, "" when unset.
	Get(key string) (string, error)
	// Set stages a value for a known key; Save persists it.
	Set(key, value string) error
	// Save writes file-backed and staged values to Path. Environment
	// overrides are never written.
	Save() error
	// Path is the config file location.
	Path() string
}

// Keys lists the top-level config keys.
func Keys() []string {
	return []string{KeyModel, KeyAspectRatio, KeyOutputFormat, KeyOutputCompression, KeyProvider}
}

// Dir returns the directory holding the config file: $ORIMAGE_CONFIG_DIR
// when set, else <user-config-dir>/orimage.
func Dir() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user config dir: %w", err)
	}
	return filepath.Join(base, appDir), nil
}

// FilePath returns the config file location.
func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// store is the viper-backed Config implementation.
type store struct {
	// main layers environment variables over the config file and serves
	// all reads. file holds only file-backed and explicitly Set values,
	// so Save never persists environment overrides.
	main *viper.Viper
	file *viper.Viper
	path string
}

var _ Config = (*store)(nil)

// New loads the configuration. A missing config file is not an error.
//
//nolint:ireturn // constructor deliberately returns the mockable interface
func New() (Config, error) {
	path, err := FilePath()
	if err != nil {
		return nil, err
	}
	main := viper.New()
	main.SetEnvPrefix(envPrefix)
	main.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	main.AutomaticEnv()
	for _, key := range Keys() {
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
	return &store{main: main, file: file, path: path}, nil
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

func (s *store) Get(key string) (string, error) {
	if err := validateScalarKey(key); err != nil {
		return "", err
	}
	return s.main.GetString(key), nil
}

func (s *store) Set(key, value string) error {
	if err := validateScalarKey(key); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(s.path), dirPerm); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := s.file.WriteConfigAs(s.path); err != nil {
		return fmt.Errorf("writing config %s: %w", s.path, err)
	}
	return nil
}

func (s *store) Path() string { return s.path }

// validateScalarKey accepts known top-level keys and provider.<setting>
// paths; the bare provider map is not scalar-addressable.
func validateScalarKey(key string) error {
	if key == KeyProvider {
		return fmt.Errorf("%q holds a settings map: address %s.<setting> instead", KeyProvider, KeyProvider)
	}
	if slices.Contains(Keys(), key) || strings.HasPrefix(key, KeyProvider+".") {
		return nil
	}
	return fmt.Errorf("unknown config key %q (known keys: %s)", key, strings.Join(Keys(), ", "))
}
