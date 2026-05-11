package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/voice/domain"
	"github.com/foodsea/core/internal/modules/voice/handler"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeUseCase struct {
	items     []domain.VoiceItem
	unmatched []string
	err       error
	gotText   string
	gotLocale string
}

func (f *fakeUseCase) Execute(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error) {
	f.gotText = text
	f.gotLocale = locale
	return f.items, f.unmatched, f.err
}

func body(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

func newRouter(h *handler.Handler) *gin.Engine {
	r := gin.New()
	r.POST("/api/v1/voice/parse", h.ParseVoice)
	return r
}

func TestHandlerHappyPath(t *testing.T) {
	uc := &fakeUseCase{items: []domain.VoiceItem{{
		ProductID:   "x",
		ProductName: "Молоко",
		Quantity:    2,
		Unit:        "шт",
		Confidence:  0.9,
		RawQuery:    "молоко",
	}}}
	h := handler.NewHandler(uc)
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: "два молока", Locale: "ru-RU"}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"product_id":"x"`) {
		t.Fatalf("body missing product: %s", rr.Body.String())
	}
	if uc.gotText != "два молока" {
		t.Fatalf("use case got text=%q", uc.gotText)
	}
	if uc.gotLocale != "ru-RU" {
		t.Fatalf("use case got locale=%q", uc.gotLocale)
	}
}

func TestHandlerDefaultsLocale(t *testing.T) {
	uc := &fakeUseCase{items: []domain.VoiceItem{}}
	h := handler.NewHandler(uc)
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: "хлеб"}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if uc.gotLocale != "ru-RU" {
		t.Fatalf("expected default locale ru-RU, got %q", uc.gotLocale)
	}
}

func TestHandlerRejectsEmpty(t *testing.T) {
	h := handler.NewHandler(&fakeUseCase{})
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: ""}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestHandlerRejectsUnsupportedLocale(t *testing.T) {
	h := handler.NewHandler(&fakeUseCase{})
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: "молоко", Locale: "en-US"}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestHandlerMLDownReturns503(t *testing.T) {
	uc := &fakeUseCase{err: errors.New("ml unavailable")}
	h := handler.NewHandler(uc)
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: "молоко"}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsBadJSON(t *testing.T) {
	h := handler.NewHandler(&fakeUseCase{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/parse", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestHandlerEnvelopeShape(t *testing.T) {
	uc := &fakeUseCase{
		items:     []domain.VoiceItem{{ProductID: "p1", ProductName: "Хлеб", Quantity: 1, Unit: "шт", Confidence: 0.8, RawQuery: "хлеб"}},
		unmatched: []string{"foo"},
	}
	h := handler.NewHandler(uc)
	r := newRouter(h)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/parse",
		body(t, handler.ParseVoiceRequestDTO{Text: "хлеб"}),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var envelope struct {
		Data handler.ParseVoiceResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "p1", envelope.Data.Items[0].ProductID)
	require.Equal(t, []string{"foo"}, envelope.Data.UnmatchedQueries)
}
