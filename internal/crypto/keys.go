// Package crypto implements envvar's versioned, crypto-agile secret envelope.
//
// The default scheme ("hybrid-x25519-mlkem768/v1") encrypts each value once with
// AES-256-GCM under a random per-value data key, then wraps that data key once
// per recipient using a hybrid KEM: classical X25519 ECDH combined with
// post-quantum ML-KEM-768. Both shared secrets are fed through HKDF, so a value
// stays secret as long as *either* primitive holds — defending against both
// "harvest now, decrypt later" quantum attacks and a future break in the
// still-young PQC scheme.
package crypto

import (
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// AlgHybridX25519MLKEM768 is the default, post-quantum-hybrid scheme id.
	AlgHybridX25519MLKEM768 = "hybrid-x25519-mlkem768/v1"

	pubKeyPrefix  = "pk_"
	privKeyPrefix = "sk_"

	x25519PubLen  = 32
	x25519PrivLen = 32
	mlkemEKLen    = 1184 // ML-KEM-768 encapsulation key
	mlkemSeedLen  = 64   // ML-KEM-768 decapsulation key seed
)

var b64 = base64.RawURLEncoding

// Recipient is a public identity: anyone holding it can encrypt to it, but
// cannot decrypt. It is safe to commit to git.
type Recipient struct {
	x25519 *ecdh.PublicKey
	mlkem  *mlkem.EncapsulationKey768
}

// Identity is a private key able to decrypt values encrypted to its Recipient.
// It must never be committed; store it in .env.keys (0600) or a platform secret.
type Identity struct {
	x25519 *ecdh.PrivateKey
	mlkem  *mlkem.DecapsulationKey768
}

// GenerateIdentity creates a fresh hybrid keypair.
func GenerateIdentity() (*Identity, error) {
	xPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("x25519 keygen: %w", err)
	}
	mlkemDK, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("ml-kem keygen: %w", err)
	}
	return &Identity{x25519: xPriv, mlkem: mlkemDK}, nil
}

// Recipient derives the public Recipient for this Identity.
func (id *Identity) Recipient() *Recipient {
	return &Recipient{
		x25519: id.x25519.PublicKey(),
		mlkem:  id.mlkem.EncapsulationKey(),
	}
}

// ID is a short, stable fingerprint (16 hex chars) used to tag which recipient a
// wrapped data key belongs to, so decryption can pick the right block.
func (r *Recipient) ID() string {
	sum := sha256.Sum256(r.rawBytes())
	return hex.EncodeToString(sum[:8])
}

func (r *Recipient) rawBytes() []byte {
	out := make([]byte, 0, x25519PubLen+mlkemEKLen)
	out = append(out, r.x25519.Bytes()...)
	out = append(out, r.mlkem.Bytes()...)
	return out
}

// String encodes the recipient as "pk_<base64>" for envvar.toml / file headers.
func (r *Recipient) String() string {
	return pubKeyPrefix + b64.EncodeToString(r.rawBytes())
}

// ParseRecipient parses a "pk_..." string.
func ParseRecipient(s string) (*Recipient, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, pubKeyPrefix) {
		return nil, fmt.Errorf("recipient: missing %q prefix", pubKeyPrefix)
	}
	raw, err := b64.DecodeString(strings.TrimPrefix(s, pubKeyPrefix))
	if err != nil {
		return nil, fmt.Errorf("recipient: bad base64: %w", err)
	}
	if len(raw) != x25519PubLen+mlkemEKLen {
		return nil, fmt.Errorf("recipient: want %d bytes, got %d", x25519PubLen+mlkemEKLen, len(raw))
	}
	xPub, err := ecdh.X25519().NewPublicKey(raw[:x25519PubLen])
	if err != nil {
		return nil, fmt.Errorf("recipient: x25519: %w", err)
	}
	ek, err := mlkem.NewEncapsulationKey768(raw[x25519PubLen:])
	if err != nil {
		return nil, fmt.Errorf("recipient: ml-kem: %w", err)
	}
	return &Recipient{x25519: xPub, mlkem: ek}, nil
}

// String encodes the private identity as "sk_<base64>" for .env.keys.
func (id *Identity) String() string {
	raw := make([]byte, 0, x25519PrivLen+mlkemSeedLen)
	raw = append(raw, id.x25519.Bytes()...)
	raw = append(raw, id.mlkem.Bytes()...)
	return privKeyPrefix + b64.EncodeToString(raw)
}

// ParseIdentity parses a "sk_..." string.
func ParseIdentity(s string) (*Identity, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, privKeyPrefix) {
		return nil, fmt.Errorf("identity: missing %q prefix", privKeyPrefix)
	}
	raw, err := b64.DecodeString(strings.TrimPrefix(s, privKeyPrefix))
	if err != nil {
		return nil, fmt.Errorf("identity: bad base64: %w", err)
	}
	if len(raw) != x25519PrivLen+mlkemSeedLen {
		return nil, fmt.Errorf("identity: want %d bytes, got %d", x25519PrivLen+mlkemSeedLen, len(raw))
	}
	xPriv, err := ecdh.X25519().NewPrivateKey(raw[:x25519PrivLen])
	if err != nil {
		return nil, fmt.Errorf("identity: x25519: %w", err)
	}
	dk, err := mlkem.NewDecapsulationKey768(raw[x25519PrivLen:])
	if err != nil {
		return nil, fmt.Errorf("identity: ml-kem: %w", err)
	}
	return &Identity{x25519: xPriv, mlkem: dk}, nil
}
