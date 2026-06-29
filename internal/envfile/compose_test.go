package envfile

import "testing"

func TestComposeResolvesReferences(t *testing.T) {
	in := map[string]string{
		"DB_USER":      "admin",
		"DB_PASS":      "s3cr3t",
		"DATABASE_URL": "postgres://{{DB_USER}}:{{DB_PASS}}@localhost/app",
	}
	out, err := Compose(in)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	want := "postgres://admin:s3cr3t@localhost/app"
	if out["DATABASE_URL"] != want {
		t.Fatalf("DATABASE_URL = %q, want %q", out["DATABASE_URL"], want)
	}
}

func TestComposeChainedReferences(t *testing.T) {
	in := map[string]string{
		"HOST": "db.internal",
		"DSN":  "host={{HOST}}",
		"URL":  "x://{{DSN}}",
	}
	out, err := Compose(in)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if out["URL"] != "x://host=db.internal" {
		t.Fatalf("URL = %q", out["URL"])
	}
}

func TestComposeUnknownReferenceLeftVerbatim(t *testing.T) {
	in := map[string]string{"A": "value-with-{{UNDEFINED}}-braces"}
	out, err := Compose(in)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if out["A"] != "value-with-{{UNDEFINED}}-braces" {
		t.Fatalf("unknown ref should be untouched, got %q", out["A"])
	}
}

func TestComposeCycleErrors(t *testing.T) {
	in := map[string]string{
		"A": "{{B}}",
		"B": "{{A}}",
	}
	if _, err := Compose(in); err == nil {
		t.Fatal("expected a cycle error, got nil")
	}
}

func TestComposeDoesNotMutateInput(t *testing.T) {
	in := map[string]string{"A": "1", "B": "{{A}}"}
	_, err := Compose(in)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if in["B"] != "{{A}}" {
		t.Fatalf("input mutated: B = %q", in["B"])
	}
}
