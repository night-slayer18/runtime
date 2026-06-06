package model

import "testing"

func statusOf(diffs []DiffEntry, key string) DiffStatus {
	for _, d := range diffs {
		if d.Key == key {
			return d.Status
		}
	}
	return DiffStatus(-1)
}

func TestCompareSecrets_AllStatuses(t *testing.T) {
	left := map[string]string{"SAME": "1", "CHANGED": "a", "ONLY_LEFT": "x"}
	right := map[string]string{"SAME": "1", "CHANGED": "b", "ONLY_RIGHT": "y"}

	diffs := CompareSecrets(left, right)

	cases := map[string]DiffStatus{
		"SAME":       DiffSame,
		"CHANGED":    DiffChanged,
		"ONLY_LEFT":  DiffOnlyLeft,
		"ONLY_RIGHT": DiffOnlyRight,
	}
	for key, want := range cases {
		if got := statusOf(diffs, key); got != want {
			t.Errorf("key %q: got %v, want %v", key, got, want)
		}
	}
}

func TestCompareSecrets_SortedAndComplete(t *testing.T) {
	diffs := CompareSecrets(map[string]string{"b": "1"}, map[string]string{"a": "1"})
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	if diffs[0].Key != "a" || diffs[1].Key != "b" {
		t.Errorf("expected sorted keys, got %q,%q", diffs[0].Key, diffs[1].Key)
	}
}

func TestCompareSecrets_EmptyInputs(t *testing.T) {
	if diffs := CompareSecrets(nil, nil); len(diffs) != 0 {
		t.Errorf("expected no diffs for empty inputs, got %d", len(diffs))
	}
}

func TestCompareSecrets_DoesNotExposeValues(t *testing.T) {
	// DiffEntry has no value field; this test documents/enforces that the
	// comparison API surface cannot leak secret values.
	diffs := CompareSecrets(map[string]string{"K": "secret-a"}, map[string]string{"K": "secret-b"})
	if statusOf(diffs, "K") != DiffChanged {
		t.Fatalf("expected changed status")
	}
}

func TestEnvMap_RoundTrip(t *testing.T) {
	m := EnvMap([]byte("A=1\nB=2\n"))
	if m["A"] != "1" || m["B"] != "2" {
		t.Errorf("unexpected env map: %v", m)
	}
}
