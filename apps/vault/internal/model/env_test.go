package model

import "testing"

func TestParseEnv_ValidEntries(t *testing.T) {
	data := []byte("# comment\nFOO=bar\nexport BAZ=\"qux\"\nEMPTY=\nQUOTED='hello world'\n")
	entries, issues := ParseEnv(data)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Key] = e.Value
	}
	want := map[string]string{
		"FOO":    "bar",
		"BAZ":    "qux",
		"EMPTY":  "",
		"QUOTED": "hello world",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %q: got %q, want %q", k, got[k], v)
		}
	}
}

func TestParseEnv_MalformedLineReported(t *testing.T) {
	data := []byte("VALID=1\nthis-has-no-equals\n123BAD=x\n")
	entries, issues := ParseEnv(data)
	if len(entries) != 1 || entries[0].Key != "VALID" {
		t.Fatalf("expected only VALID entry, got %+v", entries)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %v", len(issues), issues)
	}
}

func TestInspectEnv_DuplicateKey(t *testing.T) {
	insp := inspectEnv("test.env", []byte("A=1\nA=2\n"))
	if insp.Valid {
		t.Error("expected invalid inspection due to duplicate key")
	}
	if len(insp.Issues) == 0 {
		t.Error("expected duplicate-key issue")
	}
}

func TestInspectEnv_DoesNotEchoValues(t *testing.T) {
	secret := "super-secret-value"
	insp := inspectEnv("test.env", []byte("TOKEN="+secret+"\n"))
	for _, f := range insp.Fields {
		if f.Value == secret {
			t.Fatalf("inspection leaked secret value in field %q", f.Key)
		}
	}
}

func TestValidateEnvSchema_RequiredKeys(t *testing.T) {
	data := []byte("DB_HOST=localhost\nDB_PORT=5432\n")
	if err := ValidateEnvSchema(data, "DB_HOST", "DB_PORT"); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
	if err := ValidateEnvSchema(data, "DB_HOST", "DB_PASSWORD"); err == nil {
		t.Error("expected error for missing required key DB_PASSWORD")
	}
}
