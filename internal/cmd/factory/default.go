package factory

import (
	"github.com/schmitthub/openrouter-image-cli/internal/cmdutil"
	"github.com/schmitthub/openrouter-image-cli/internal/iostreams"
	"github.com/schmitthub/openrouter-image-cli/internal/openrouter"
)

func New(appVersion string, ios *iostreams.IOStreams) *cmdutil.Factory {
	return &cmdutil.Factory{
		AppVersion: appVersion,
		IOStreams:  ios,
		OpenRouter: openRouterFunc(),
	}
}

func openRouterFunc() func() (*openrouter.Client, error) {
	return func() (*openrouter.Client, error) {
		return openrouter.NewFromEnv(
			openrouter.WithAttribution(
				"https://github.com/schmitthub/openrouter-image-cli", "orimage"))
	}
}
