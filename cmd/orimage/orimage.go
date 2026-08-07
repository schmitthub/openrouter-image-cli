package main

import (
	"os"

	"github.com/schmitthub/openrouter-image-cli/internal/orimagecmd"
)

func main() {
	code := orimagecmd.Main()
	os.Exit(int(code))
}
