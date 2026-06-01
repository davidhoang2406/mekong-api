package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/app"
	"github.com/davidhoang2406/mekong-api/internal/config"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestApp() *app.App {
	return &app.App{
		Cache: store.NewCache(time.Minute),
		Cfg: config.Config{
			MinioAnalysisBucket: "test-bucket",
		},
	}
}

func doRequest(r http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	r.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

// ── /api/v1/ohlcv ────────────────────────────────────────────────────────────

func TestGetOHLCV_MissingSymbol(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/ohlcv")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if body["code"] != "MISSING_PARAM" {
		t.Errorf("code = %v", body["code"])
	}
}

// ── /api/v1/indicators ───────────────────────────────────────────────────────

func TestGetIndicators_MissingSymbol(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/indicators")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if body["code"] != "MISSING_PARAM" {
		t.Errorf("code = %v", body["code"])
	}
}

// ── /api/v1/digest ────────────────────────────────────────────────────────────

func TestGetDigest_InvalidLimit(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/digest?limit=abc")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if body["code"] != "INVALID_PARAM" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestGetDigest_ZeroLimit(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/digest?limit=0")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetDigest_InvalidDate(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/digest?date=not-a-date")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if body["code"] != "INVALID_PARAM" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestGetDigest_NegativeLimit(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/digest?limit=-5")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ── /api/v1/snapshot ─────────────────────────────────────────────────────────

func TestGetSnapshot_MissingSymbol(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/snapshot")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetSnapshot_NoWSURL(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/snapshot?symbol=VCB")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	body := decodeBody(t, w)
	if body["code"] != "NOT_CONFIGURED" {
		t.Errorf("code = %v", body["code"])
	}
}

// ── /api/v1/symbols — cache hit ───────────────────────────────────────────────

func TestGetSymbols_CacheHit(t *testing.T) {
	a := newTestApp()
	a.Cache.Set("symbols:", gin.H{"symbols": []string{"VCB", "FPT"}})

	r := a.SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/symbols")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// ── error envelope shape ──────────────────────────────────────────────────────

func TestErrorEnvelope_HasRequiredFields(t *testing.T) {
	r := newTestApp().SetupRouter()
	w := doRequest(r, http.MethodGet, "/api/v1/ohlcv")
	body := decodeBody(t, w)
	for _, field := range []string{"error", "code", "status"} {
		if body[field] == nil {
			t.Errorf("missing field %q in error envelope", field)
		}
	}
}
