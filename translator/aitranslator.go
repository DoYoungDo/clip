package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type AITranslator struct {
	isEnabled bool
	secret    string
	baseURL   string
	model     string
}

var defaultAITranslatorSecret = "sk-vnCe39ImYmru7sYViwRjbmeMQzLTMaeKQo14QhU1M55vVyYb"
var defaultAITranslatorBaseURL = "https://api.agicto.cn/v1"
var defaultAITranslatorModel = "ERNIE-Speed-128K"

// Secret implements [Translator].
func (a *AITranslator) Secret() string {
	if !a.isEnabled {
		return ""
	}

	return a.secret
}

// Id implements [Translator].
func (a *AITranslator) Id() string {
	return "AITranslator"
}

func NewAITranslator() *AITranslator {
	secret := defaultAITranslatorSecret
	return &AITranslator{
		isEnabled: secret != "",
		secret:    secret,
		baseURL:   defaultAITranslatorBaseURL,
		model:     defaultAITranslatorModel,
	}
}

// Enable implements [Translator].
func (a *AITranslator) Enable(secret string) bool {
	_ = secret
	a.isEnabled = a.secret != ""
	return a.isEnabled
}

// IsEnabled implements [Translator].
func (a *AITranslator) IsEnabled() bool {
	return a.isEnabled
}

// Name implements [Translator].
func (a *AITranslator) Name() string {
	name := "AI 翻译"
	return name
}

// Translate implements [Translator].
func (a *AITranslator) Translate(text string, lang TransLang) (string, error) {
	if !a.isEnabled {
		return "", fmt.Errorf("AI 翻译器未启用")
	}
	if strings.TrimSpace(a.baseURL) == "" {
		return "", fmt.Errorf("AI 翻译器未配置 baseURL")
	}
	if strings.TrimSpace(a.model) == "" {
		return "", fmt.Errorf("AI 翻译器未配置模型")
	}

	body, err := json.Marshal(aiTranslatorChatRequest{
		Messages: []aiTranslatorChatMessage{
			{
				Role:    "system",
				Content: "你是一名资深的翻译专家，你精通多国语言，你能做到将多国语言都表达得和母语者一样，不管要你翻译什么文本，你都必须以`翻译结果为：xxx`为格式进行输出，并且不能私自添加一些其它的字符或标点符号。",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("将`%s`翻译为%s", text, aiTranslatorLangName(lang)),
			},
		},
		Model:            a.model,
		Stream:           false,
		MaxTokens:        1024,
		Temperature:      0.2,
		TopP:             0.7,
		TopK:             50,
		FrequencyPenalty: 1,
	})
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(a.baseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.secret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败: http %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result aiTranslatorChatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("请求失败: 未返回翻译结果")
	}

	translated := strings.TrimSpace(result.Choices[0].Message.Content)
	if translated == "" {
		return "", fmt.Errorf("请求失败: 翻译结果为空")
	}
	translated = strings.TrimPrefix(translated, "翻译结果为：")
	translated = strings.TrimSpace(translated)
	if translated == "" {
		return "", fmt.Errorf("请求失败: 翻译结果为空")
	}

	return translated, nil

}

type aiTranslatorChatRequest struct {
	Messages         []aiTranslatorChatMessage `json:"messages"`
	Model            string                    `json:"model"`
	Stream           bool                      `json:"stream"`
	MaxTokens        int                       `json:"max_tokens"`
	Temperature      float64                   `json:"temperature"`
	TopP             float64                   `json:"top_p"`
	TopK             int                       `json:"top_k"`
	FrequencyPenalty float64                   `json:"frequency_penalty"`
}

type aiTranslatorChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiTranslatorChatResponse struct {
	Choices []aiTranslatorChatChoice `json:"choices"`
}

type aiTranslatorChatChoice struct {
	Message aiTranslatorChatMessage `json:"message"`
}

func aiTranslatorLangName(lang TransLang) string {
	switch lang {
	case ZH:
		return "Simplified Chinese"
	case EN:
		return "English"
	default:
		return string(lang)
	}
}

var _ Translator = (*AITranslator)(nil)
