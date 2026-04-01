package translator

import "sync"

type TransLang string

const (
	ZH TransLang = "zh"
	EN TransLang = "en"
)

type Translator interface {
	Name() string
	Translate(text string, lang TransLang) (string, error)
	Enable(secret string) bool
	IsEnabled() bool
}

var TranslatorFactory = sync.OnceValue(func() []Translator {
	return []Translator{
		NewBaiduTranslator(),
	}
})

func TransLangFactory() []TransLang {
	return []TransLang{
		ZH,
		EN,
	}
}
