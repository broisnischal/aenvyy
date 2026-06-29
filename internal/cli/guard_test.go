package cli

import (
	"strings"
	"testing"
)

func TestScanEnvForLeaks(t *testing.T) {
	content := []byte(strings.Join([]string{
		`DOTENV_PUBLIC_KEY_ALG="hybrid-x25519-mlkem768/v1"`,
		`ENCRYPTED="enc:v1:abc:def:personal.xyz"`,
		`PLAINTEXT=oops-this-is-raw`,
		`LEAKED_KEY=sk_AAAAfakekeymaterial`,
		`EMPTY=`,
	}, "\n"))

	problems := scanEnvForLeaks(".env", content)

	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "PLAINTEXT") {
		t.Errorf("expected PLAINTEXT flagged; got: %v", problems)
	}
	if !strings.Contains(joined, "LEAKED_KEY") {
		t.Errorf("expected LEAKED_KEY flagged as private key; got: %v", problems)
	}
	if strings.Contains(joined, "ENCRYPTED") {
		t.Errorf("encrypted value should not be flagged; got: %v", problems)
	}
	if strings.Contains(joined, "DOTENV_") {
		t.Errorf("header keys should be ignored; got: %v", problems)
	}
	if strings.Contains(joined, "EMPTY") {
		t.Errorf("empty value should not be flagged; got: %v", problems)
	}
}

func TestScanEnvCleanFileHasNoProblems(t *testing.T) {
	content := []byte("KEY=\"enc:v1:n:c:personal.w\"\n# a comment\n\n")
	if p := scanEnvForLeaks(".env.production", content); len(p) != 0 {
		t.Fatalf("clean file flagged: %v", p)
	}
}

func TestIsSecretEnvFile(t *testing.T) {
	cases := map[string]bool{
		".env":             true,
		".env.production":  true,
		".env.staging":     true,
		".env.keys":        false,
		".env.example":     false,
		".env.sample":      false,
		".env.template":    false,
		".env.local":       false,
		".env.prod.local":  false,
		"config.yaml":      false,
		"envvar.toml":      false,
	}
	for name, want := range cases {
		if got := isSecretEnvFile(name); got != want {
			t.Errorf("isSecretEnvFile(%q) = %v, want %v", name, got, want)
		}
	}
}
