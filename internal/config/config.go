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

// Provider routing keys under KeyProvider. Slug lists persist as YAML
// sequences or comma-separated strings; passthrough options nest one
// level deeper as provider.options.<slug>.<key>. Note viper lowercases
// every config key, so case-sensitive passthrough keys (e.g. Google's
// "cachedContent") cannot round-trip through the config file and must
// be passed on the command line instead.
const (
	KeyProviderSort           = "provider.sort"
	KeyProviderOrder          = "provider.order"
	KeyProviderOnly           = "provider.only"
	KeyProviderIgnore         = "provider.ignore"
	KeyProviderAllowFallbacks = "provider.allow_fallbacks"
	KeyProviderOptions        = "provider.options"
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
	// ProviderSort returns the provider sorting strategy, or "".
	ProviderSort() string
	// ProviderOrder returns the ordered provider slug list, nil when unset.
	ProviderOrder() []string
	// ProviderOnly returns the allowed provider slug list, nil when unset.
	ProviderOnly() []string
	// ProviderIgnore returns the excluded provider slug list, nil when unset.
	ProviderIgnore() []string
	// ProviderAllowFallbacks returns the fallback setting and whether one
	// is configured.
	ProviderAllowFallbacks() (bool, bool)
	// ProviderOptions returns per-provider passthrough parameters keyed by
	// provider slug, nil when unset.
	ProviderOptions() map[string]map[string]any
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

func (s *store) ProviderSort() string { return s.main.GetString(KeyProviderSort) }

func (s *store) ProviderOrder() []string  { return s.providerList(KeyProviderOrder) }
func (s *store) ProviderOnly() []string   { return s.providerList(KeyProviderOnly) }
func (s *store) ProviderIgnore() []string { return s.providerList(KeyProviderIgnore) }

// providerList reads a slug list stored either as a YAML sequence or as
// a comma-separated string (the form Set and ORGEN_* environment
// variables use).
func (s *store) providerList(key string) []string {
	switch v := s.main.Get(key).(type) {
	case nil:
		return nil
	case string:
		return splitList(v)
	default:
		return s.main.GetStringSlice(key)
	}
}

// splitList splits a comma-separated slug list, dropping empty entries.
func splitList(v string) []string {
	var out []string
	for part := range strings.SplitSeq(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *store) ProviderAllowFallbacks() (bool, bool) {
	if !s.main.IsSet(KeyProviderAllowFallbacks) {
		return false, false
	}
	return s.main.GetBool(KeyProviderAllowFallbacks), true
}

func (s *store) ProviderOptions() map[string]map[string]any {
	raw := s.main.GetStringMap(KeyProviderOptions)
	if len(raw) == 0 {
		return nil
	}
	options := make(map[string]map[string]any, len(raw))
	for slug, v := range raw {
		if m, ok := v.(map[string]any); ok {
			options[slug] = m
		}
	}
	return options
}

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
	val, err := coerceValue(key, value)
	if err != nil {
		return err
	}
	s.main.Set(key, val)
	s.file.Set(key, val)
	return nil
}

// coerceValue converts a value to the native type its key persists as:
// output_compression an integer, provider.allow_fallbacks a boolean,
// provider.options.* best-effort scalars so passthrough parameters
// reach the wire typed. Everything else stays a string.
func coerceValue(key, value string) (any, error) {
	switch {
	case key == KeyOutputCompression:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("value for %s must be an integer, got %q", key, value)
		}
		return n, nil
	case key == KeyProviderAllowFallbacks:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("value for %s must be a boolean, got %q", key, value)
		}
		return b, nil
	case strings.HasPrefix(key, KeyProviderOptions+"."):
		return CoerceScalar(value), nil
	default:
		return value, nil
	}
}

// CoerceScalar best-effort converts a string to an integer, float, or
// boolean, returning the string unchanged when it is none of those.
// Passthrough parameter values use it so numbers reach the API as
// numbers rather than quoted strings.
func CoerceScalar(value string) any {
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	return value
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
	if rest, ok := strings.CutPrefix(key, KeyProviderOptions); ok &&
		(rest == "" || strings.HasPrefix(rest, ".")) &&
		!strings.Contains(strings.TrimPrefix(rest, "."), ".") {
		return fmt.Errorf(
			"%q nests per-provider maps: address %s.<slug>.<key> instead", key, KeyProviderOptions)
	}
	if slices.Contains(s.main.AllKeys(), key) || strings.HasPrefix(key, KeyProvider+".") {
		return nil
	}
	return fmt.Errorf("unknown config key %q (known keys: %s)", key, strings.Join(s.AllKeys(), ", "))
}
