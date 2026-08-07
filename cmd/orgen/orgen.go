package main

import (
	"os"

	"github.com/schmitthub/openrouter-generate/internal/orgencmd"
)

func main() {
	code := orgencmd.Main()
	os.Exit(int(code))
}
