package governance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMiddleware_OverLimitBodyRejectedNotForwarded verifies that when a
// RequestBuilder reads the body, an over-limit request is rejected with HTTP
// 400 (parse error) instead of being read unbounded, and the downstream
// handler is never invoked.
func TestMiddleware_OverLimitBodyRejectedNotForwarded(t *testing.T) {
	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// RequestBuilder reads the full body; this is the read MaxBytesReader caps.
	builder := func(r *http.Request) (*GovernanceRequest, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		return &GovernanceRequest{ToolName: "noop", RawPayload: body}, nil
	}

	p := NewPipeline(PipelineConfig{}, nil, nil, nil)
	mw := NewMiddleware(p, next, MiddlewareConfig{
		RequestBuilder:  builder,
		MaxRequestBytes: 64, // small cap to keep the test cheap
	})

	oversized := strings.Repeat("A", 4096)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversized))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for over-limit body; body=%s", rec.Code, rec.Body.String())
	}
	if nextCalled {
		t.Fatal("downstream handler must not run when the body is rejected")
	}
}

// TestMiddleware_BodyWithinLimitForwarded is the control: a body under the cap
// is read by the RequestBuilder and forwarded to the downstream handler.
func TestMiddleware_BodyWithinLimitForwarded(t *testing.T) {
	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	builder := func(r *http.Request) (*GovernanceRequest, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		return &GovernanceRequest{ToolName: "noop", RawPayload: body}, nil
	}

	p := NewPipeline(PipelineConfig{}, nil, nil, nil)
	mw := NewMiddleware(p, next, MiddlewareConfig{
		RequestBuilder:  builder,
		MaxRequestBytes: 1 << 16, // 64 KiB
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatalf("within-limit body should be forwarded; status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestMiddleware_DefaultMaxRequestBytesUsedWhenUnset pins the documented
// default so an unconfigured middleware is still bounded.
func TestMiddleware_DefaultMaxRequestBytesUsedWhenUnset(t *testing.T) {
	mw := NewMiddleware(NewPipeline(PipelineConfig{}, nil, nil, nil), nil, MiddlewareConfig{})
	if got := mw.maxRequestBytes(); got != DefaultMaxRequestBytes {
		t.Fatalf("unset middleware cap = %d, want default %d", got, DefaultMaxRequestBytes)
	}
	if DefaultMaxRequestBytes != 4<<20 {
		t.Fatalf("DefaultMaxRequestBytes = %d, want 4 MiB", DefaultMaxRequestBytes)
	}
}
