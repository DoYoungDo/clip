package translator

type BaiduTranslator struct {
	isEnabled bool
}

// IsEnabled implements [Translator].
func (b *BaiduTranslator) IsEnabled() bool {
	return b.isEnabled
}

// Enable implements [Translator].
func (b *BaiduTranslator) Enable(secret string) bool {
	return false
}

// Name implements [Translator].
func (b *BaiduTranslator) Name() string {
	return "百度翻译"
}

// Translate implements [Translator].
func (b *BaiduTranslator) Translate(text string) (string, error) {
	return text + " (百度翻译)", nil
}

var _ Translator = (*BaiduTranslator)(nil)
