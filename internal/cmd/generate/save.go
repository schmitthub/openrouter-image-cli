package generate

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/schmitthub/openrouter-generate/internal/openrouter"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// mediaTypeExt maps response MIME types to file extensions. Falls back to
// formatPNG for unknown types.
var mediaTypeExt = map[string]string{ //nolint:gochecknoglobals // static lookup table
	"image/png":     formatPNG,
	"image/jpeg":    "jpg",
	"image/webp":    formatWebP,
	"image/svg+xml": formatSVG,
}

// saveImages decodes every image in resp and writes it to disk, returning
// the written paths. out semantics:
//   - "": orgen-<created>.<ext> in the current directory
//   - path with one image: written exactly there
//   - path with multiple images: index inserted before the extension
//     (img.png -> img-1.png, img-2.png, ...)
func saveImages(resp *openrouter.ImageResponse, out string) ([]string, error) {
	paths := make([]string, 0, len(resp.Data))
	for i, img := range resp.Data {
		raw, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decoding image %d: %w", i+1, err)
		}

		path := outPath(out, resp.Created, ext(img.MediaType), i, len(resp.Data))
		if dir := filepath.Dir(path); dir != "." {
			if mkErr := os.MkdirAll(dir, dirPerm); mkErr != nil {
				return nil, fmt.Errorf("creating output directory: %w", mkErr)
			}
		}
		if wrErr := os.WriteFile(path, raw, filePerm); wrErr != nil {
			return nil, fmt.Errorf("writing image: %w", wrErr)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// ext returns the file extension for a response media type.
func ext(mediaType string) string {
	if e, ok := mediaTypeExt[mediaType]; ok {
		return e
	}
	return formatPNG
}

// outPath computes the output path for image i of n.
func outPath(out string, created int64, extension string, i, n int) string {
	if out == "" {
		if n == 1 {
			return fmt.Sprintf("orgen-%d.%s", created, extension)
		}
		return fmt.Sprintf("orgen-%d-%d.%s", created, i+1, extension)
	}
	if n == 1 {
		return out
	}
	e := filepath.Ext(out)
	base := strings.TrimSuffix(out, e)
	if e == "" {
		e = "." + extension
	}
	return fmt.Sprintf("%s-%d%s", base, i+1, e)
}
