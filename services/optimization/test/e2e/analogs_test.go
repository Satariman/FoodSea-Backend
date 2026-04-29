//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestGetAnalogsHappyPath(t *testing.T) {
	resp, body := doJSONRequest(t, http.MethodGet, "/api/v1/analogs/"+productMilk.String()+"?top_k=3", authHeader(testUserID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /analogs/:id status=%d body=%s", resp.StatusCode, string(body))
	}

	var got analogsResponse
	decodeEnvelope(t, body, &got)
	if len(got.Analogs) == 0 {
		t.Fatalf("expected non-empty analogs list")
	}
	for i, analog := range got.Analogs {
		if analog.Score <= 0 {
			t.Fatalf("analog[%d] score must be > 0", i)
		}
		if analog.MinPriceKopecks <= 0 {
			t.Fatalf("analog[%d] min_price_kopecks must be > 0", i)
		}
	}
}

func TestGetAnalogsUnknownProductReturnsEmptyList(t *testing.T) {
	unknown := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	resp, body := doJSONRequest(t, http.MethodGet, "/api/v1/analogs/"+unknown.String(), authHeader(testUserID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /analogs/:id status=%d body=%s", resp.StatusCode, string(body))
	}

	var got analogsResponse
	decodeEnvelope(t, body, &got)
	if len(got.Analogs) != 0 {
		t.Fatalf("expected empty analogs list, got %d", len(got.Analogs))
	}
}
