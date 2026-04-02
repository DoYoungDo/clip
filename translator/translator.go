package translator

import "sync"

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
