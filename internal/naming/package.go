// Package naming provides shared project name normalization for Nix attributes.
package naming

import (
	"path/filepath"
	"strings"
	"unicode"
)

// PackageName normalizes a project or module name for use as a Nix pname / mainProgram.
func PackageName(name string) string {
	base := filepath.Base(name)
	base = strings.TrimSpace(strings.ToLower(base))

	var b strings.Builder
	lastDash := false
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "app-" + out
	}
	return out
}
