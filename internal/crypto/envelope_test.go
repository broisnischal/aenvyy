package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func mustIdentity(t *testing.T) *Identity {
	t.Helper()
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	return id
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	id := mustIdentity(t)
	plain := []byte("postgres://user:s3cr3t@db:5432/app")

	env, err := Encrypt(plain, []*Recipient{id.Recipient()})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !IsEncrypted(env) {
		t.Fatalf("IsEncrypted=false for %q", env)
	}
	got, err := Decrypt(env, id)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round trip mismatch: got %q want %q", got, plain)
	}
}

func TestMultiRecipientAnyKeyDecrypts(t *testing.T) {
	personal := mustIdentity(t)
	recovery := mustIdentity(t)
	stranger := mustIdentity(t)
	plain := []byte("API_KEY=sk_live_abc123")

	env, err := Encrypt(plain, []*Recipient{personal.Recipient(), recovery.Recipient()})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	for name, id := range map[string]*Identity{"personal": personal, "recovery": recovery} {
		got, err := Decrypt(env, id)
		if err != nil {
			t.Fatalf("Decrypt with %s key: %v", name, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("%s: mismatch", name)
		}
	}

	if _, err := Decrypt(env, stranger); err != ErrNoMatchingRecipient {
		t.Fatalf("stranger should not decrypt: got err=%v", err)
	}
}

func TestKeySerializationRoundTrip(t *testing.T) {
	id := mustIdentity(t)

	id2, err := ParseIdentity(id.String())
	if err != nil {
		t.Fatalf("ParseIdentity: %v", err)
	}
	r2, err := ParseRecipient(id.Recipient().String())
	if err != nil {
		t.Fatalf("ParseRecipient: %v", err)
	}
	if id.Recipient().ID() != r2.ID() {
		t.Fatalf("recipient id mismatch after parse")
	}

	// Encrypt to the parsed recipient, decrypt with the parsed identity.
	env, err := Encrypt([]byte("hello"), []*Recipient{r2})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := Decrypt(env, id2)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("mismatch: %q", got)
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	id := mustIdentity(t)
	env, err := Encrypt([]byte("secret"), []*Recipient{id.Recipient()})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// Flip a character in the ciphertext field.
	tampered := strings.Replace(env, "enc:v1:", "enc:v1:", 1)
	parts := strings.SplitN(tampered, ":", 5)
	b := []byte(parts[3])
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	parts[3] = string(b)
	if _, err := Decrypt(strings.Join(parts, ":"), id); err == nil {
		t.Fatalf("expected AEAD failure on tampered ciphertext")
	}
}
