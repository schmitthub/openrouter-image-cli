// Package openrouter is a minimal client for the OpenRouter REST API,
// covering the endpoints this CLI needs. API reference:
// https://openrouter.ai/docs/api/api-reference/overview
package openrouter

import (
	"errors"
	"net/http"
	"os"
	"time"
)

// DefaultBaseURL is the production OpenRouter API root.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// defaultTimeout bounds a single API call. Image generation is slow —
// minutes, not seconds — so this is deliberately generous; pass a context
// with a deadline to GenerateImage for tighter control.
const defaultTimeout = 5 * time.Minute

// Client calls the OpenRouter API. Construct with New.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	// Optional attribution headers, surfaced on openrouter.ai rankings.
	// https://openrouter.ai/docs/api/reference/overview#headers
	referer string
	title   string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (primarily for tests).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithAttribution sets the optional HTTP-Referer and X-Title app
// attribution headers.
func WithAttribution(referer, title string) Option {
	return func(c *Client) {
		c.referer = referer
		c.title = title
	}
}

// EnvAPIKey is the environment variable holding the OpenRouter API key.
const EnvAPIKey = "OPENROUTER_API_KEY" //nolint:gosec // env var name, not a credential

// NewFromEnv returns a Client authenticated with the key in $OPENROUTER_API_KEY,
// or an error when the variable is unset or empty.
func NewFromEnv(opts ...Option) (*Client, error) {
	key := os.Getenv(EnvAPIKey)
	if key == "" {
		return nil, errors.New(EnvAPIKey + " environment variable is not set")
	}
	return New(key, opts...), nil
}

// New returns a Client authenticated with apiKey.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		referer:    "",
		title:      "",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
