// Package envfile parses and serializes .env-compatible files where keys stay
// readable and values may be envvar envelopes. It preserves comments, blank
// lines and key order so git diffs stay clean, and it only ever rewrites the
// values that actually change (selective re-encryption).
package envfile

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/nees/envvar/internal/crypto"
)

// Header keys recognized in the file itself.
const (
	HeaderAlg        = "DOTENV_PUBLIC_KEY_ALG"
	HeaderRecipients = "DOTENV_RECIPIENTS" // "label=pk_...,label2=pk_..."
)

type kind int

const (
	kindRaw kind = iota // comment or blank line, stored verbatim
	kindPair
)

type line struct {
	kind  kind
	key   string
	value string
	raw   string
}

// File is an ordered, round-trippable representation of an env file.
type File struct {
	lines []line
}

// Parse reads an env file. It never fails on unknown content; malformed lines
// are preserved as raw text.
func Parse(data []byte) *File {
	f := &File{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // values can be ~KB each
	for sc.Scan() {
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			f.lines = append(f.lines, line{kind: kindRaw, raw: raw})
			continue
		}
		eq := strings.IndexByte(raw, '=')
		if eq < 0 {
			f.lines = append(f.lines, line{kind: kindRaw, raw: raw})
			continue
		}
		key := strings.TrimSpace(raw[:eq])
		val := unquote(strings.TrimSpace(raw[eq+1:]))
		f.lines = append(f.lines, line{kind: kindPair, key: key, value: val})
	}
	return f
}

// Get returns the raw stored value (possibly an envelope) for key.
func (f *File) Get(key string) (string, bool) {
	for _, l := range f.lines {
		if l.kind == kindPair && l.key == key {
			return l.value, true
		}
	}
	return "", false
}

// Set inserts or updates a key with an already-prepared value (plaintext or
// envelope), preserving position for existing keys and appending new ones.
func (f *File) Set(key, value string) {
	for i := range f.lines {
		if f.lines[i].kind == kindPair && f.lines[i].key == key {
			f.lines[i].value = value
			return
		}
	}
	f.lines = append(f.lines, line{kind: kindPair, key: key, value: value})
}

// Pairs returns all key/value pairs in file order.
func (f *File) Pairs() [][2]string {
	var out [][2]string
	for _, l := range f.lines {
		if l.kind == kindPair {
			out = append(out, [2]string{l.key, l.value})
		}
	}
	return out
}

// Recipients parses the DOTENV_RECIPIENTS header into label->Recipient.
func (f *File) Recipients() (map[string]*crypto.Recipient, error) {
	v, ok := f.Get(HeaderRecipients)
	if !ok || strings.TrimSpace(v) == "" {
		return nil, nil
	}
	out := map[string]*crypto.Recipient{}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		label, pk, found := strings.Cut(part, "=")
		if !found {
			return nil, fmt.Errorf("recipients header: bad entry %q", part)
		}
		r, err := crypto.ParseRecipient(strings.TrimSpace(pk))
		if err != nil {
			return nil, fmt.Errorf("recipients header %q: %w", label, err)
		}
		out[strings.TrimSpace(label)] = r
	}
	return out, nil
}

// SetRecipients writes the header from an ordered list of (label, recipient).
func (f *File) SetRecipients(labels []string, recips map[string]*crypto.Recipient) {
	var sb strings.Builder
	for i, label := range labels {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, "%s=%s", label, recips[label].String())
	}
	f.Set(HeaderAlg, crypto.AlgHybridX25519MLKEM768)
	f.Set(HeaderRecipients, sb.String())
}

// Bytes serializes back to env-file text. Values needing quotes are quoted.
func (f *File) Bytes() []byte {
	var sb strings.Builder
	for _, l := range f.lines {
		if l.kind == kindRaw {
			sb.WriteString(l.raw)
			sb.WriteByte('\n')
			continue
		}
		fmt.Fprintf(&sb, "%s=%s\n", l.key, quote(l.value))
	}
	return []byte(sb.String())
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func quote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t#\"'$") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
