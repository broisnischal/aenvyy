package envfile

import (
	"fmt"
	"strings"

	"github.com/nees/envvar/internal/crypto"
)

// IsHeaderKey reports whether a key is envvar metadata (not a secret to encrypt).
func IsHeaderKey(key string) bool {
	return strings.HasPrefix(key, "DOTENV_")
}

// EncryptInPlace encrypts every plaintext secret value to the given recipients.
// Values that are already envelopes are left byte-for-byte unchanged, so a save
// only touches what actually changed (clean git diffs). Each newly encrypted
// value is round-trip verified against verify (may be nil to skip).
func (f *File) EncryptInPlace(recipients []*crypto.Recipient, verify *crypto.Identity) error {
	if len(recipients) == 0 {
		return fmt.Errorf("encrypt: no recipients configured")
	}
	for i := range f.lines {
		l := &f.lines[i]
		if l.kind != kindPair || IsHeaderKey(l.key) || crypto.IsEncrypted(l.value) {
			continue
		}
		env, err := crypto.Encrypt([]byte(l.value), recipients)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", l.key, err)
		}
		if verify != nil {
			got, err := crypto.Decrypt(env, verify)
			if err != nil || string(got) != l.value {
				return fmt.Errorf("encrypt %s: round-trip validation failed", l.key)
			}
		}
		l.value = env
	}
	return nil
}

// RekeyInPlace re-wraps every value under a fresh data key for the given
// recipient set. Encrypted values are decrypted with old and re-encrypted;
// plaintext values are encrypted for the first time. This powers `envvar rekey`:
// rotating data keys, switching algorithms, or granting/revoking a recipient
// (change the recipient list, then rekey so existing secrets are reachable by
// the new set). Each result is round-trip verified against verify (may be nil).
func (f *File) RekeyInPlace(old *crypto.Identity, recipients []*crypto.Recipient, verify *crypto.Identity) error {
	if len(recipients) == 0 {
		return fmt.Errorf("rekey: no recipients configured")
	}
	for i := range f.lines {
		l := &f.lines[i]
		if l.kind != kindPair || IsHeaderKey(l.key) {
			continue
		}
		plain := l.value
		if crypto.IsEncrypted(l.value) {
			b, err := crypto.Decrypt(l.value, old)
			if err != nil {
				return fmt.Errorf("rekey %s: decrypt: %w", l.key, err)
			}
			plain = string(b)
		}
		env, err := crypto.Encrypt([]byte(plain), recipients)
		if err != nil {
			return fmt.Errorf("rekey %s: %w", l.key, err)
		}
		if verify != nil {
			got, derr := crypto.Decrypt(env, verify)
			if derr != nil || string(got) != plain {
				return fmt.Errorf("rekey %s: round-trip validation failed", l.key)
			}
		}
		l.value = env
	}
	return nil
}

// SetSecret encrypts a single plaintext value and stores it under key.
func (f *File) SetSecret(key, plaintext string, recipients []*crypto.Recipient) error {
	env, err := crypto.Encrypt([]byte(plaintext), recipients)
	if err != nil {
		return err
	}
	f.Set(key, env)
	return nil
}

// Decrypted returns all secret key/value pairs as plaintext, decrypting any
// envelopes with id and passing plaintext values through. Header keys are
// excluded. Use this to build the environment for `run`.
func (f *File) Decrypted(id *crypto.Identity) (map[string]string, error) {
	out := map[string]string{}
	for _, kv := range f.Pairs() {
		key, val := kv[0], kv[1]
		if IsHeaderKey(key) {
			continue
		}
		if !crypto.IsEncrypted(val) {
			out[key] = val
			continue
		}
		plain, err := crypto.Decrypt(val, id)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s: %w", key, err)
		}
		out[key] = string(plain)
	}
	return out, nil
}
