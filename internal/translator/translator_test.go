package translator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestTranslateReturnsErrorWhenServerTimesOut(t *testing.T) {
	originalClient := httpClient
	httpClient = &http.Client{Timeout: 50 * time.Millisecond}
	defer func() { httpClient = originalClient }()

	originalEndpoint := baiduTranslateEndpoint
	defer func() { baiduTranslateEndpoint = originalEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"from":"auto","to":"zh","trans_result":[{"src":"hello","dst":"你好"}]}`))
	}))
	defer server.Close()

	baiduTranslateEndpoint = server.URL
	b := NewBaiduTranslator()
	b.appid = "appid"
	b.key = "key"

	start := time.Now()
	_, err := b.Translate("hello", ZH)
	if err == nil {
		t.Fatalf("服务端响应超时时应返回错误")
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Fatalf("应受客户端超时限制，实际耗时 %v", elapsed)
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
