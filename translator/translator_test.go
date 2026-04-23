package translator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAITranslatorEnableStoresSecret(t *testing.T) {
	originalSecret := defaultAITranslatorSecret
	defaultAITranslatorSecret = "built-in-secret"
	defer func() { defaultAITranslatorSecret = originalSecret }()

	a := NewAITranslator()

	if !a.Enable("") {
		t.Fatalf("AI 翻译器启用后应返回 true")
	}
	if !a.IsEnabled() {
		t.Fatalf("AI 翻译器应处于启用状态")
	}
	if got := a.Secret(); got != "built-in-secret" {
		t.Fatalf("应保存启用时传入的密钥，got %q", got)
	}
}

func TestAITranslatorTranslateUsesOpenAICompatibleAPI(t *testing.T) {
	originalSecret := defaultAITranslatorSecret
	originalBaseURL := defaultAITranslatorBaseURL
	originalModel := defaultAITranslatorModel
	defaultAITranslatorSecret = "built-in-secret"
	defaultAITranslatorModel = "ERNIE-Speed-128K"
	defer func() {
		defaultAITranslatorSecret = originalSecret
		defaultAITranslatorBaseURL = originalBaseURL
		defaultAITranslatorModel = originalModel
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("请求方法错误: %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("请求路径错误: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer built-in-secret" {
			t.Fatalf("Authorization 错误: %q", got)
		}

		var req aiTranslatorChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if req.Model != "ERNIE-Speed-128K" {
			t.Fatalf("模型错误: %q", req.Model)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("消息数量错误: %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "翻译结果为：xxx") {
			t.Fatalf("system 消息错误: %#v", req.Messages[0])
		}
		if req.Messages[1].Role != "user" || req.Messages[1].Content != "将`hello`翻译为Simplified Chinese" {
			t.Fatalf("user 消息错误: %#v", req.Messages[1])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"翻译结果为：你好"}}]}`))
	}))
	defer server.Close()

	defaultAITranslatorBaseURL = server.URL
	a := NewAITranslator()

	translated, err := a.Translate("hello", ZH)
	if err != nil {
		t.Fatalf("AI 翻译请求不应报错: %v", err)
	}
	if translated != "你好" {
		t.Fatalf("翻译结果错误: %q", translated)
	}
}

func TestAITranslatorTranslateReturnsErrorWhenChoicesEmpty(t *testing.T) {
	originalSecret := defaultAITranslatorSecret
	originalBaseURL := defaultAITranslatorBaseURL
	defaultAITranslatorSecret = "built-in-secret"
	defer func() {
		defaultAITranslatorSecret = originalSecret
		defaultAITranslatorBaseURL = originalBaseURL
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	defaultAITranslatorBaseURL = server.URL
	a := NewAITranslator()

	_, err := a.Translate("hello", ZH)
	if err == nil {
		t.Fatalf("choices 为空时应返回错误")
	}
	if !strings.Contains(err.Error(), "未返回翻译结果") {
		t.Fatalf("错误信息应包含未返回翻译结果，got %v", err)
	}
}

func TestBaiduTranslateReturnsErrorWhenAPIRejectsRequest(t *testing.T) {
	originalEndpoint := baiduTranslateEndpoint
	defer func() { baiduTranslateEndpoint = originalEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error_code":"54001","error_msg":"Invalid Sign"}`))
	}))
	defer server.Close()

	baiduTranslateEndpoint = server.URL
	b := NewBaiduTranslator()
	b.appid = "appid"
	b.key = "key"

	_, err := b.Translate("hello", ZH)
	if err == nil {
		t.Fatalf("百度接口返回错误码时应返回错误")
	}
	if !strings.Contains(err.Error(), "54001") {
		t.Fatalf("错误信息应包含百度错误码，got %v", err)
	}
}

func TestBaiduTranslateReturnsErrorWhenResultIsEmpty(t *testing.T) {
	originalEndpoint := baiduTranslateEndpoint
	defer func() { baiduTranslateEndpoint = originalEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"from":"auto","to":"zh","trans_result":[]}`))
	}))
	defer server.Close()

	baiduTranslateEndpoint = server.URL
	b := NewBaiduTranslator()
	b.appid = "appid"
	b.key = "key"

	_, err := b.Translate("hello", ZH)
	if err == nil {
		t.Fatalf("百度接口未返回翻译结果时应返回错误")
	}
}

func TestYoudaoTranslateReturnsErrorOnHTTPFailure(t *testing.T) {
	originalEndpoint := youdaoTranslateEndpoint
	defer func() { youdaoTranslateEndpoint = originalEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	youdaoTranslateEndpoint = server.URL
	y := NewYouDaoTranslator()
	y.appkey = "appkey"
	y.secret = "secret"

	_, err := y.Translate("hello", ZH)
	if err == nil {
		t.Fatalf("有道接口返回非 200 状态码时应返回错误")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("错误信息应包含 HTTP 状态码，got %v", err)
	}
}
