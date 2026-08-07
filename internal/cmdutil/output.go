package cmdutil

import (
	"bytes"
	"encoding/json"
)

// JSONStringify renders v as JSON, indented for humans by default or on a
// single line when compact is set.
func JSONStringify(v any, compact bool) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if !compact {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return b.String(), nil
}
