package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
)

// envelopePrefix tags an encrypted value. Format (all binary fields base64url):
//
//	enc:v1:<nonce>:<ciphertext>:<recipientBlock>[;<recipientBlock>...]
//
// recipientBlock = <recipientID>.<kemCT>.<wrapNonce>.<wrappedDataKey>
//
// The leading "enc:v1" makes the scheme self-describing so new algorithm
// versions can coexist with old ciphertext (crypto-agility).
const envelopePrefix = "enc:v1"

const (
	dataKeyLen   = 32 // AES-256
	hkdfInfo     = "envvar/v1 " + AlgHybridX25519MLKEM768
	mlkemCTLen   = 1088 // ML-KEM-768 ciphertext
)

// ErrNoMatchingRecipient means none of the wrapped data keys were addressed to
// the identity that attempted decryption.
var ErrNoMatchingRecipient = errors.New("crypto: no wrapped key for this identity")

// IsEncrypted reports whether a value string is an envvar envelope.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, envelopePrefix+":")
}

// Encrypt seals plaintext to every recipient. Any one corresponding Identity can
// later decrypt it. Recipients must be non-empty.
func Encrypt(plaintext []byte, recipients []*Recipient) (string, error) {
	if len(recipients) == 0 {
		return "", errors.New("crypto: at least one recipient required")
	}

	dataKey := make([]byte, dataKeyLen)
	if _, err := rand.Read(dataKey); err != nil {
		return "", err
	}

	nonce, ciphertext, err := aeadSeal(dataKey, plaintext)
	if err != nil {
		return "", fmt.Errorf("seal value: %w", err)
	}

	blocks := make([]string, 0, len(recipients))
	for _, r := range recipients {
		kemCT, kek, err := kemEncapsulate(r)
		if err != nil {
			return "", fmt.Errorf("encapsulate to %s: %w", r.ID(), err)
		}
		wrapNonce, wrapped, err := aeadSeal(kek, dataKey)
		if err != nil {
			return "", fmt.Errorf("wrap data key: %w", err)
		}
		blocks = append(blocks, strings.Join([]string{
			r.ID(),
			b64.EncodeToString(kemCT),
			b64.EncodeToString(wrapNonce),
			b64.EncodeToString(wrapped),
		}, "."))
	}

	return strings.Join([]string{
		envelopePrefix,
		b64.EncodeToString(nonce),
		b64.EncodeToString(ciphertext),
		strings.Join(blocks, ";"),
	}, ":"), nil
}

// Decrypt opens an envelope using the given identity.
func Decrypt(envelope string, id *Identity) ([]byte, error) {
	// enc:v1:<nonce>:<ciphertext>:<blocks>
	parts := strings.SplitN(envelope, ":", 5)
	if len(parts) != 5 || parts[0]+":"+parts[1] != envelopePrefix {
		return nil, errors.New("crypto: not a valid enc:v1 envelope")
	}
	nonce, err := b64.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := b64.DecodeString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	wantID := id.Recipient().ID()
	for _, block := range strings.Split(parts[4], ";") {
		f := strings.Split(block, ".")
		if len(f) != 4 || f[0] != wantID {
			continue
		}
		kemCT, err := b64.DecodeString(f[1])
		if err != nil {
			return nil, fmt.Errorf("decode kem ct: %w", err)
		}
		wrapNonce, err := b64.DecodeString(f[2])
		if err != nil {
			return nil, fmt.Errorf("decode wrap nonce: %w", err)
		}
		wrapped, err := b64.DecodeString(f[3])
		if err != nil {
			return nil, fmt.Errorf("decode wrapped key: %w", err)
		}
		kek, err := kemDecapsulate(id, kemCT)
		if err != nil {
			return nil, fmt.Errorf("decapsulate: %w", err)
		}
		dataKey, err := aeadOpen(kek, wrapNonce, wrapped)
		if err != nil {
			return nil, fmt.Errorf("unwrap data key: %w", err)
		}
		return aeadOpen(dataKey, nonce, ciphertext)
	}
	return nil, ErrNoMatchingRecipient
}

// kemEncapsulate runs the hybrid KEM against a recipient public key, returning
// the combined KEM ciphertext (ephemeral X25519 pub || ML-KEM ct) and the
// derived 32-byte key-encryption key.
func kemEncapsulate(r *Recipient) (kemCT, kek []byte, err error) {
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	ssX, err := eph.ECDH(r.x25519)
	if err != nil {
		return nil, nil, err
	}
	ssM, ctM := r.mlkem.Encapsulate()

	kek, err = deriveKEK(ssX, ssM)
	if err != nil {
		return nil, nil, err
	}
	kemCT = append(append([]byte{}, eph.PublicKey().Bytes()...), ctM...)
	return kemCT, kek, nil
}

func kemDecapsulate(id *Identity, kemCT []byte) (kek []byte, err error) {
	if len(kemCT) != x25519PubLen+mlkemCTLen {
		return nil, fmt.Errorf("kem ct: want %d bytes, got %d", x25519PubLen+mlkemCTLen, len(kemCT))
	}
	ephPub, err := ecdh.X25519().NewPublicKey(kemCT[:x25519PubLen])
	if err != nil {
		return nil, err
	}
	ssX, err := id.x25519.ECDH(ephPub)
	if err != nil {
		return nil, err
	}
	ssM, err := id.mlkem.Decapsulate(kemCT[x25519PubLen:])
	if err != nil {
		return nil, err
	}
	return deriveKEK(ssX, ssM)
}

// deriveKEK binds both shared secrets together via HKDF-SHA256.
func deriveKEK(ssX, ssM []byte) ([]byte, error) {
	combined := append(append([]byte{}, ssX...), ssM...)
	return hkdf.Key(sha256.New, combined, nil, hkdfInfo, dataKeyLen)
}

func aeadSeal(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, nil), nil
}

func aeadOpen(key, nonce, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
