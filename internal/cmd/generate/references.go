package generate

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxInputReferences mirrors the API's limit on input_references.
const maxInputReferences = 16

// resolveInputReferences expands local file paths into base64 data URIs.
// HTTP(S) URLs, data URIs, and raw base64 values pass through unchanged.
func resolveInputReferences(refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	resolved := make([]string, len(refs))
	for i, ref := range refs {
		r, err := resolveReference(ref)
		if err != nil {
			return nil, err
		}
		resolved[i] = r
	}
	return resolved, nil
}

func resolveReference(ref string) (string, error) {
	lower := strings.ToLower(ref)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "data:") {
		return ref, nil
	}

	if info, statErr := os.Stat(ref); statErr == nil {
		if info.IsDir() {
			return "", fmt.Errorf("input-reference %q is a directory", ref)
		}
		data, err := os.ReadFile(ref)
		if err != nil {
			return "", fmt.Errorf("reading input-reference %q: %w", ref, err)
		}
		return "data:" + detectMediaType(ref, data) + ";base64," +
			base64.StdEncoding.EncodeToString(data), nil
	}

	// Not a file on disk: treat as raw base64 and let the API judge.
	return ref, nil
}

// detectMediaType sniffs the file content, falling back to the extension
// for types the sniffer can't identify (e.g. SVG reads as text/plain).
func detectMediaType(path string, data []byte) string {
	sniffed := http.DetectContentType(data)
	if sniffed != "application/octet-stream" && !strings.HasPrefix(sniffed, "text/") {
		return sniffed
	}
	if byExt := mime.TypeByExtension(filepath.Ext(path)); byExt != "" {
		return byExt
	}
	return sniffed
}
