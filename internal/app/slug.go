package app

import "strings"

// Slugify turns a display name into an Account.ID like "lean-tech" (PRD
// §17 example: displayName "Lean Tech" → id "lean-tech"). It's exported so
// the CLI layer can suggest matching defaults (e.g. a key filename)
// without duplicating this logic.
func Slugify(displayName string) string {
	var b strings.Builder
	lastWasHyphen := true // avoid a leading hyphen
	for _, r := range strings.ToLower(displayName) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasHyphen = false
		default:
			if !lastWasHyphen {
				b.WriteByte('-')
				lastWasHyphen = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
