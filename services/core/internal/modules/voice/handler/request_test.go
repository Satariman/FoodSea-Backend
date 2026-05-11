package handler

import (
	"strings"
	"testing"
)

func TestValidateAcceptsShortRussianText(t *testing.T) {
	r := ParseVoiceRequestDTO{Text: "молоко", Locale: "ru-RU"}
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	r := ParseVoiceRequestDTO{Text: ""}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsTooLong(t *testing.T) {
	r := ParseVoiceRequestDTO{Text: strings.Repeat("a", 501)}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsUnknownLocale(t *testing.T) {
	r := ParseVoiceRequestDTO{Text: "x", Locale: "en-US"}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLocaleDefaultsToRu(t *testing.T) {
	r := ParseVoiceRequestDTO{Text: "x"}
	if r.LocaleOrDefault() != "ru-RU" {
		t.Fatalf("got %s", r.LocaleOrDefault())
	}
}
