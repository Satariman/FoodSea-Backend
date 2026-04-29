//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestOptimizeHappyPathAndCacheHit(t *testing.T) {
	first := postOptimize(t, testUserID)
	if len(first.Items) == 0 {
		t.Fatalf("expected non-empty items")
	}
	if first.TotalKopecks <= 0 {
		t.Fatalf("expected total_kopecks > 0, got %d", first.TotalKopecks)
	}
	if len(first.Substitutions) == 0 {
		t.Fatalf("expected at least one substitution")
	}

	second := postOptimize(t, testUserID)
	if first.ID != second.ID {
		t.Fatalf("expected cache hit with same result id, got %s vs %s", first.ID, second.ID)
	}
}

func TestGetOptimizeResultByID(t *testing.T) {
	created := postOptimize(t, testUserID)

	resp, body := doJSONRequest(t, http.MethodGet, "/api/v1/optimize/"+created.ID, authHeader(testUserID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /optimize/:id status=%d body=%s", resp.StatusCode, string(body))
	}

	var got optimizationResultResponse
	decodeEnvelope(t, body, &got)
	if got.ID != created.ID {
		t.Fatalf("expected id %s, got %s", created.ID, got.ID)
	}
}

func TestGetOptimizeResultForDifferentUserIsNotFound(t *testing.T) {
	created := postOptimize(t, testUserID)

	resp, body := doJSONRequest(t, http.MethodGet, "/api/v1/optimize/"+created.ID, authHeader(otherUserID))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestOptimizeWithoutToken(t *testing.T) {
	resp, body := doJSONRequest(t, http.MethodPost, "/api/v1/optimize", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", resp.StatusCode, string(body))
	}
}
