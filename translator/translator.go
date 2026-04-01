package translator

type Translator interface {
	Name() string
	Translate(text string) (string, error)
	Enable(secret string) bool
	IsEnabled() bool
}

func TranslatorFactory() []Translator {
	return []Translator{
		&BaiduTranslator{},
	}
}
