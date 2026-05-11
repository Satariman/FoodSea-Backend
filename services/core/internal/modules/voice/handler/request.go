package handler

import "errors"

type ParseVoiceRequestDTO struct {
	Text   string `json:"text"`
	Locale string `json:"locale"`
}

const (
	maxTextLen = 500
	minTextLen = 1
)

var allowedLocales = map[string]struct{}{
	"ru-RU": {},
}

func (r ParseVoiceRequestDTO) Validate() error {
	if len(r.Text) < minTextLen {
		return errors.New("text is required")
	}
	if len(r.Text) > maxTextLen {
		return errors.New("text exceeds 500 characters")
	}
	if r.Locale == "" {
		return nil // default ru-RU applied later
	}
	if _, ok := allowedLocales[r.Locale]; !ok {
		return errors.New("unsupported locale")
	}
	return nil
}

func (r ParseVoiceRequestDTO) LocaleOrDefault() string {
	if r.Locale == "" {
		return "ru-RU"
	}
	return r.Locale
}
