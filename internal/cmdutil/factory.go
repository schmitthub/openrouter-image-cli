package cmdutil

import (
	"github.com/schmitthub/openrouter-generate/internal/config"
	"github.com/schmitthub/openrouter-generate/internal/iostreams"
	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

type Factory struct {
	AppVersion string
	IOStreams  *iostreams.IOStreams

	// OpenRouter returns an authenticated API client, or an error when no
	// credentials are available. Deferred behind a func so commands that
	// never touch the API (version, help) work without a key.
	OpenRouter func() (*openrouter.Client, error)

	// Config lazily loads persisted settings; the loaded value is
	// memoized, so repeated calls share one Config.
	Config func() (config.Config, error)
}
