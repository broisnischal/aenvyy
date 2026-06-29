package envfile

import (
	"fmt"
	"regexp"
)

// refPattern matches a secret reference like {{DB_PASSWORD}} (optional inner
// whitespace). Reference names follow shell-variable rules.
var refPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Compose resolves {{KEY}} references between decrypted values, enabling
// composition like DATABASE_URL=postgres://{{DB_USER}}:{{DB_PASS}}@host/db.
//
// A reference to a key present in the map is replaced with that key's fully
// resolved value (references may chain). A reference to an unknown key is left
// verbatim, so a value that merely contains braces is never corrupted. The only
// error is a reference cycle.
func Compose(in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}

	resolving := map[string]bool{}
	resolved := map[string]bool{}

	var resolve func(key string) (string, error)
	resolve = func(key string) (string, error) {
		if resolved[key] {
			return out[key], nil
		}
		if resolving[key] {
			return "", fmt.Errorf("compose: reference cycle through %q", key)
		}
		resolving[key] = true

		var cbErr error
		expanded := refPattern.ReplaceAllStringFunc(out[key], func(match string) string {
			name := refPattern.FindStringSubmatch(match)[1]
			if _, ok := out[name]; !ok {
				return match // unknown reference: leave untouched
			}
			rv, err := resolve(name)
			if err != nil {
				cbErr = err
				return match
			}
			return rv
		})
		if cbErr != nil {
			return "", cbErr
		}

		out[key] = expanded
		resolving[key] = false
		resolved[key] = true
		return expanded, nil
	}

	for k := range out {
		if _, err := resolve(k); err != nil {
			return nil, err
		}
	}
	return out, nil
}
