package translator

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var baiduTranslateEndpoint = "https://fanyi-api.baidu.com/api/trans/vip/translate"

type BaiduTranslator struct {
	isEnabled bool
	appid     string
	key       string
}

// Secret implements [Translator].
func (b *BaiduTranslator) Secret() string {
	if !b.isEnabled {
		return ""
	}

	return fmt.Sprintf("%v %v", b.appid, b.key)
}

// Id implements [Translator].
func (b *BaiduTranslator) Id() string {
	return "BaiduTranslator"
}

func NewBaiduTranslator() *BaiduTranslator {
	return &BaiduTranslator{
		isEnabled: false,
		appid:     "",
		key:       "",
	}
}

// IsEnabled implements [Translator].
func (b *BaiduTranslator) IsEnabled() bool {
	return b.isEnabled
}

// Enable implements [Translator].
func (b *BaiduTranslator) Enable(secret string) bool {
	lines := strings.Split(secret, " ")
	if len(lines) != 2 {
		return false
	}
	b.appid = lines[0]
	b.key = lines[1]

	_, err := b.Translate("hello", "zh")
	if err != nil {
		return false
	}

	b.isEnabled = true
	return b.isEnabled
}

// Name implements [Translator].
func (b *BaiduTranslator) Name() string {
	name := "百度翻译"
	if !b.isEnabled {
		return fmt.Sprintf("%v%v", name, "，请确保当前剪切板的内容为：appid 密钥")
	}
	return name
}

type BaiduTranslationResult struct {
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	Result    []struct {
		Src string `json:"src"`
		Dis string `json:"dst"`
	} `json:"trans_result"`
}

// Translate implements [Translator].
func (b *BaiduTranslator) Translate(text string, lang TransLang) (string, error) {
	salt := time.Now().UnixMilli()
	signText := fmt.Sprintf("%v%v%v%v", b.appid, text, salt, b.key)
	sign := fmt.Sprintf("%x", md5.Sum([]byte(signText)))

	p := url.Values{}
	p.Set("q", text)
	p.Set("from", "auto")
	p.Set("to", string(lang))
	p.Set("appid", b.appid)
	p.Set("salt", fmt.Sprintf("%v", salt))
	p.Set("sign", fmt.Sprintf("%v", sign))

	resp, err := http.Get(fmt.Sprintf("%s?%v", baiduTranslateEndpoint, p.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败: http %d", resp.StatusCode)
	}

	var result BaiduTranslationResult

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	if result.ErrorCode != "" {
		return "", fmt.Errorf("请求失败: %s %s", result.ErrorCode, result.ErrorMsg)
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("请求失败: 未返回翻译结果")
	}

	translations := make([]string, len(result.Result))
	for i, item := range result.Result {
		translations[i] = item.Dis
	}

	return strings.Join(translations, "\n"), nil
}

var _ Translator = (*BaiduTranslator)(nil)
