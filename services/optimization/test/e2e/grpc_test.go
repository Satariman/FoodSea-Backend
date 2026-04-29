//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbopt "github.com/foodsea/proto/optimization"
)

func TestGRPCLockUnlockGetResult(t *testing.T) {
	created := postOptimize(t, testUserID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lockResp, err := optGRPCClient.LockResult(ctx, &pbopt.LockResultRequest{ResultId: created.ID})
	if err != nil {
		t.Fatalf("LockResult failed: %v", err)
	}
	if !lockResp.GetSuccess() {
		t.Fatalf("expected lock success=true")
	}

	_, err = optGRPCClient.LockResult(ctx, &pbopt.LockResultRequest{ResultId: created.ID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition on second lock, got err=%v", err)
	}

	unlockResp, err := optGRPCClient.UnlockResult(ctx, &pbopt.UnlockResultRequest{ResultId: created.ID})
	if err != nil {
		t.Fatalf("UnlockResult failed: %v", err)
	}
	if !unlockResp.GetSuccess() {
		t.Fatalf("expected unlock success=true")
	}

	resp, body := doJSONRequest(t, http.MethodGet, "/api/v1/optimize/"+created.ID, authHeader(testUserID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /optimize/:id after unlock status=%d body=%s", resp.StatusCode, string(body))
	}
	var httpResult optimizationResultResponse
	decodeEnvelope(t, body, &httpResult)
	if httpResult.Status != "active" {
		t.Fatalf("expected status=active, got %q", httpResult.Status)
	}

	getResp, err := optGRPCClient.GetResult(ctx, &pbopt.GetResultRequest{ResultId: created.ID})
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if getResp.GetResult() == nil {
		t.Fatalf("expected result in GetResult response")
	}
	if len(getResp.GetResult().GetItems()) == 0 {
		t.Fatalf("expected result items in GetResult response")
	}
}
