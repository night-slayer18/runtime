package model

import (
	"strings"
	"testing"
)

func TestDetectAPIKeyFormat_KnownFormats(t *testing.T) {
	cases := map[string]string{
		"AKIAIOSFODNN7EXAMPLE":                     "AWS Access Key ID",
		"ghp_0123456789abcdefghijklmnopqrstuvwxyz": "GitHub Token",
		"550e8400-e29b-41d4-a716-446655440000":     "UUID Token",
	}
	for key, want := range cases {
		got, ok := DetectAPIKeyFormat(key)
		if !ok || got != want {
			t.Errorf("key %q: got (%q,%v), want %q", key, got, ok, want)
		}
	}
}

func TestDetectAPIKeyFormat_Generic(t *testing.T) {
	got, ok := DetectAPIKeyFormat("some-random-unstructured-value")
	if ok {
		t.Errorf("expected generic (unrecognised), got %q", got)
	}
	if got != "Generic" {
		t.Errorf("expected name Generic, got %q", got)
	}
}

func TestInspectAPIKey_ShortKeyFlagged(t *testing.T) {
	insp := inspectAPIKey("<input>", "short")
	if insp.Valid {
		t.Error("expected short key to be invalid")
	}
}

func TestInspectAPIKey_DoesNotEchoKey(t *testing.T) {
	key := "ghp_0123456789abcdefghijklmnopqrstuvwxyz"
	insp := inspectAPIKey("<input>", key)
	for _, f := range insp.Fields {
		if strings.Contains(f.Value, key) {
			t.Fatalf("API key leaked in field %q", f.Key)
		}
	}
}
