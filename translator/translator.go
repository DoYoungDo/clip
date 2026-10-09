package translator

import (
	"net/http"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type TransLang string

const (
	ZH TransLang = "zh"
	EN TransLang = "en"
)

type Translator interface {
	Id() string
	Name() string
	Translate(text string, lang TransLang) (string, error)
	Enable(secret string) bool
	IsEnabled() bool
	Secret() string
}

var TranslatorFactory = sync.OnceValue(func() []Translator {
	return []Translator{
		NewAITranslator(),
		NewBaiduTranslator(),
		NewYouDaoTranslator(),
	}
})

func TransLangFactory() []TransLang {
	return []TransLang{
		ZH,
		EN,
	}
}
