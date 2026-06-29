package envfile

import (
	"testing"

	"github.com/nees/envvar/internal/crypto"
)

func TestRekeyGrantsNewRecipientAccess(t *testing.T) {
	alice, err := crypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := crypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	// Start with a value encrypted to Alice only.
	f := Parse(nil)
	if err := f.SetSecret("API_KEY", "top-secret", []*crypto.Recipient{alice.Recipient()}); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	// Bob cannot read it yet.
	if _, err := f.Decrypted(bob); err == nil {
		t.Fatal("bob should not decrypt before rekey")
	}

	// Rekey to Alice + Bob, decrypting with Alice's key.
	recips := []*crypto.Recipient{alice.Recipient(), bob.Recipient()}
	if err := f.RekeyInPlace(alice, recips, alice); err != nil {
		t.Fatalf("RekeyInPlace: %v", err)
	}

	// Now both can read it.
	for name, id := range map[string]*crypto.Identity{"alice": alice, "bob": bob} {
		vals, err := f.Decrypted(id)
		if err != nil {
			t.Fatalf("%s decrypt: %v", name, err)
		}
		if vals["API_KEY"] != "top-secret" {
			t.Fatalf("%s got %q", name, vals["API_KEY"])
		}
	}
}

func TestRekeyEncryptsPlaintextValues(t *testing.T) {
	id, err := crypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	f := Parse([]byte("PLAIN=hello\n"))
	if err := f.RekeyInPlace(id, []*crypto.Recipient{id.Recipient()}, id); err != nil {
		t.Fatalf("RekeyInPlace: %v", err)
	}
	v, _ := f.Get("PLAIN")
	if !crypto.IsEncrypted(v) {
		t.Fatalf("PLAIN should be encrypted after rekey, got %q", v)
	}
}

func TestEncryptInPlaceLeavesEnvelopesUnchanged(t *testing.T) {
	id, err := crypto.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	recips := []*crypto.Recipient{id.Recipient()}
	f := Parse([]byte("A=plain\n"))
	if err := f.EncryptInPlace(recips, id); err != nil {
		t.Fatal(err)
	}
	first, _ := f.Get("A")
	// A second pass must not re-encrypt (stable output → clean git diffs).
	if err := f.EncryptInPlace(recips, id); err != nil {
		t.Fatal(err)
	}
	second, _ := f.Get("A")
	if first != second {
		t.Fatal("re-encrypting an already-encrypted value changed it")
	}
}
