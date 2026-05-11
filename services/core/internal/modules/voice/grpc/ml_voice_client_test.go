package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"

	pbml "github.com/foodsea/proto/ml"
)

type fakeStub struct {
	resp   *pbml.ParseShoppingListResponse
	err    error
	gotReq *pbml.ParseShoppingListRequest
}

func (f *fakeStub) ParseShoppingList(_ context.Context, req *pbml.ParseShoppingListRequest, _ ...grpc.CallOption) (*pbml.ParseShoppingListResponse, error) {
	f.gotReq = req
	return f.resp, f.err
}

func TestParseShoppingListMapsResponse(t *testing.T) {
	stub := &fakeStub{resp: &pbml.ParseShoppingListResponse{
		Items: []*pbml.VoiceItem{
			{ProductId: "x", ProductName: "Молоко", Quantity: 2, Unit: "л", Confidence: 0.9, RawQuery: "молоко"},
		},
		UnmatchedQueries: []string{"foo"},
	}}
	c := &MLVoiceClient{client: stub, timeout: time.Second}
	items, unmatched, err := c.ParseShoppingList(context.Background(), "молоко", "ru-RU")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 1 || items[0].ProductID != "x" || items[0].Quantity != 2 {
		t.Fatalf("unexpected items: %+v", items)
	}
	if items[0].ProductName != "Молоко" || items[0].Unit != "л" || items[0].Confidence != 0.9 || items[0].RawQuery != "молоко" {
		t.Fatalf("voice item fields not mapped: %+v", items[0])
	}
	if len(unmatched) != 1 || unmatched[0] != "foo" {
		t.Fatalf("unexpected unmatched: %+v", unmatched)
	}
	if stub.gotReq.GetText() != "молоко" || stub.gotReq.GetLocale() != "ru-RU" {
		t.Fatalf("request not propagated: %+v", stub.gotReq)
	}
	if stub.gotReq.GetTopKPerItem() != 1 {
		t.Fatalf("expected TopKPerItem=1, got %d", stub.gotReq.GetTopKPerItem())
	}
}

func TestParseShoppingListPropagatesError(t *testing.T) {
	stub := &fakeStub{err: errors.New("ml down")}
	c := &MLVoiceClient{client: stub, timeout: time.Second}
	_, _, err := c.ParseShoppingList(context.Background(), "x", "ru-RU")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
