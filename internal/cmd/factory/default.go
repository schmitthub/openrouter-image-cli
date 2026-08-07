package factory

import (
	"sync"

	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/config"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

func New(appVersion string, ios *iostreams.IOStreams) *cmdutil.Factory {
	return &cmdutil.Factory{
		AppVersion: appVersion,
		IOStreams:  ios,
		OpenRouter: openRouterFunc(),
		Config:     configFunc(),
	}
}

// configFunc defers config loading until first use and memoizes the
// result, so keyless commands never touch the filesystem.
func configFunc() func() (config.Config, error) {
	return sync.OnceValues(config.New)
}

func openRouterFunc() func() (*openrouter.Client, error) {
	return func() (*openrouter.Client, error) {
		return openrouter.NewFromEnv(
			openrouter.WithAttribution(
				"https://github.com/schmitthub/openrouter-image-cli", "orimage"))
	}
}
