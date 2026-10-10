package madriver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestLiveUpstreamNeverFollowsCrossHostRedirect proves the live upstream client
// refuses to follow a redirect to a different host: server A answers every
// request with a 307 pointing at server B, and B must never see the request
// (which would carry the x-api-key header).
func TestLiveUpstreamNeverFollowsCrossHostRedirect(t *testing.T) {
	var bHits atomic.Int32
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bHits.Add(1)
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, `{"id":"sess_b","agent":"agt"}`)
	}))
	defer srvB.Close()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvB.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srvA.Close()

	up := newLiveUpstream(srvA.URL, "fake")
	defer up.Close()
	_, err := up.CreateSession(context.Background(), CreateSessionParams{AgentID: "a", EnvironmentID: "e", BudgetUSD: 1})
	if err == nil {
		t.Fatal("a redirecting upstream must produce an error, not a session")
	}
	if got := bHits.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0 (x-api-key must never leave the live origin)", got)
	}
}

// TestLiveUpstreamNeverFollowsSameHostRedirect proves redirects are refused
// even to a different path on the same host: the redirect target would serve a
// valid response if followed, so a returned error means it was not.
func TestLiveUpstreamNeverFollowsSameHostRedirect(t *testing.T) {
	var targetHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/real/v1/sessions" {
			targetHits.Add(1)
			w.Header().Set("content-type", "application/json")
			fmt.Fprint(w, `{"id":"sess_real","agent":"agt"}`)
			return
		}
		http.Redirect(w, r, "/real"+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	up := newLiveUpstream(srv.URL, "fake")
	defer up.Close()
	_, err := up.CreateSession(context.Background(), CreateSessionParams{AgentID: "a", EnvironmentID: "e", BudgetUSD: 1})
	if err == nil {
		t.Fatal("a redirecting upstream must produce an error, not a session")
	}
	if got := targetHits.Load(); got != 0 {
		t.Fatalf("same-host redirect target received %d requests, want 0", got)
	}
}
