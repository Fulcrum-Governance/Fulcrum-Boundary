package governance

import "testing"

func TestCanonicalJSONBytesMatchesInternal(t *testing.T) {
	v := map[string]any{"b": 1, "a": "<x>&", "n": 1.5}
	got := string(CanonicalJSONBytes(v))
	want := string(mustCanonicalJSON(v))
	if got != want {
		t.Fatalf("CanonicalJSONBytes != mustCanonicalJSON:\n got=%s\nwant=%s", got, want)
	}
	// JCS sorts keys and leaves <,>,& literal.
	if want != `{"a":"<x>&","b":1,"n":1.5}` {
		t.Fatalf("unexpected JCS form: %s", want)
	}
}
