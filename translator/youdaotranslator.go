package translator

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type YouDaoTranslator struct {
	isEnabled bool
	appkey    string
	secret    string
}

func NewYouDaoTranslator() *YouDaoTranslator {
	return &YouDaoTranslator{
		isEnabled: false,
		appkey:    "",
		secret:    "",
	}
}

// Enable implements [Translator].
func (y *YouDaoTranslator) Enable(secret string) bool {
	lines := strings.Split(secret, " ")
	if len(lines) != 2 {
		return false
	}
	y.appkey = lines[0]
	y.secret = lines[1]

	_, err := y.Translate("hello", "zh")
	if err != nil {
		return false
	}

	y.isEnabled = true
	return y.isEnabled
}

// IsEnabled implements [Translator].
func (y *YouDaoTranslator) IsEnabled() bool {
	return y.isEnabled
}

// Name implements [Translator].
func (y *YouDaoTranslator) Name() string {
	name := "有道翻译"
	if !y.isEnabled {
		return fmt.Sprintf("%v，请确保当前剪切板的内容为：appkey 密钥", name)
	}
	return name
}

type youDaoTranslationResult struct {
	ErrorCode   string   `json:"errorCode"`
	Translation []string `json:"translation"`
}

// Translate implements [Translator].
func (y *YouDaoTranslator) Translate(text string, lang TransLang) (string, error) {
	salt := int32(time.Now().Unix())
	time.Now().UTC()
	signText := fmt.Sprintf("%v%v%v%v%v", y.appkey, text, salt, salt, y.secret)
	sign := fmt.Sprintf("%x", sha256.Sum256([]byte(signText)))

	p := url.Values{}
	p.Set("q", text)
	p.Set("from", "auto")
	p.Set("to", langToString(lang))
	p.Set("appKey", y.appkey)
	p.Set("salt", fmt.Sprintf("%v", salt))
	p.Set("sign", fmt.Sprintf("%v", sign))
	p.Set("signType", "v3")
	p.Set("curtime", fmt.Sprintf("%v", salt))

	resp, err := http.Get(fmt.Sprintf("https://openapi.youdao.com/api?%v", p.Encode()))
	if err != nil {
		return "", err
	}
	var result youDaoTranslationResult

	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &result)
	if err != nil || result.ErrorCode != "0" {
		return "", fmt.Errorf("请求失败")
	}

	return strings.Join(result.Translation, "\n"), nil
}

func langToString(lang TransLang) string {
	switch lang {
	case ZH:
		return "zh-CHS"
	}
	return string(lang)
}

var _ Translator = (*YouDaoTranslator)(nil)
