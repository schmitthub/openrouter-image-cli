package generate

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxInputReferences mirrors the API's limit on input_references.
const maxInputReferences = 16

// maxPathLen and maxPathComponent bound what mainstream filesystems accept
// for a path and a single path component. Values beyond these limits cannot
// name a file, so they skip the filesystem probe entirely — raw base64
// payloads for real images routinely exceed them and would otherwise turn
// [os.Stat]'s ENAMETOOLONG into a spurious rejection.
const (
	maxPathLen       = 4096
	maxPathComponent = 255
)

// looksLikePath reports whether ref could plausibly name a file on disk.
func looksLikePath(ref string) bool {
	if len(ref) > maxPathLen {
		return false
	}
	parts := strings.FieldsFunc(ref, func(r rune) bool {
		return r == '/' || r == os.PathSeparator
	})
	for _, part := range parts {
		if len(part) > maxPathComponent {
			return false
		}
	}
	return true
}

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

	var statErr error
	if looksLikePath(ref) {
		var info os.FileInfo
		info, statErr = os.Stat(ref)
		switch {
		case statErr == nil:
			return encodeFile(ref, info)
		case !errors.Is(statErr, fs.ErrNotExist):
			// EACCES on an ancestor directory, ELOOP, EIO: the value names
			// something real that cannot be inspected. Never guess base64
			// here — surface the filesystem error verbatim.
			return "", fmt.Errorf("input-reference %q: %w", ref, statErr)
		}
	}

	// A nonexistent path or raw base64 is all that remains; raw base64 is
	// the only valid form. Report both underlying errors so the failure is
	// never ambiguous.
	if _, decodeErr := base64.StdEncoding.DecodeString(ref); decodeErr != nil {
		if statErr != nil {
			return "", fmt.Errorf("input-reference %q: %w; not valid base64: %w", ref, statErr, decodeErr)
		}
		return "", fmt.Errorf("input-reference %q is not valid base64: %w", ref, decodeErr)
	}
	return ref, nil
}

// encodeFile reads a stat-confirmed file and renders it as a base64 data URI.
func encodeFile(ref string, info os.FileInfo) (string, error) {
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
