package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/foodsea/core/internal/modules/voice/domain"
)

type fakeMLClient struct {
	items     []domain.VoiceItem
	unmatched []string
	err       error
	gotText   string
	gotLocale string
}

func (f *fakeMLClient) ParseShoppingList(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error) {
	f.gotText, f.gotLocale = text, locale
	return f.items, f.unmatched, f.err
}

func TestExecuteCallsMLAndReturnsResult(t *testing.T) {
	ml := &fakeMLClient{
		items:     []domain.VoiceItem{{ProductID: "x", Quantity: 1}},
		unmatched: []string{"foo"},
	}
	uc := NewParseVoice(ml)
	items, unmatched, err := uc.Execute(context.Background(), "молоко", "ru-RU")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(items) != 1 || items[0].ProductID != "x" {
		t.Fatalf("items wrong: %+v", items)
	}
	if len(unmatched) != 1 {
		t.Fatalf("unmatched wrong: %+v", unmatched)
	}
	if ml.gotText != "молоко" || ml.gotLocale != "ru-RU" {
		t.Fatalf("propagation: %+v", ml)
	}
}

func TestExecutePropagatesError(t *testing.T) {
	ml := &fakeMLClient{err: errors.New("boom")}
	uc := NewParseVoice(ml)
	_, _, err := uc.Execute(context.Background(), "x", "ru-RU")
	if err == nil {
		t.Fatal("expected error")
	}
}
