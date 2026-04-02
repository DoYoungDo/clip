package main

import (
	"errors"
	"testing"

	"clip/translator"
)

type fakeTranslator struct {
	id      string
	name    string
	secret  string
	enabled bool
}

func (f *fakeTranslator) Id() string { return f.id }

func (f *fakeTranslator) Name() string { return f.name }

func (f *fakeTranslator) Translate(text string, lang translator.TransLang) (string, error) {
	return text, nil
}

func (f *fakeTranslator) Enable(secret string) bool {
	f.secret = secret
	f.enabled = secret != ""
	return f.enabled
}

func (f *fakeTranslator) IsEnabled() bool { return f.enabled }

func (f *fakeTranslator) Secret() string { return f.secret }

type memorySecretStore struct {
	secrets map[string]string
	getErr  map[string]error
	setErr  map[string]error
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{
		secrets: map[string]string{},
		getErr:  map[string]error{},
		setErr:  map[string]error{},
	}
}

func (m *memorySecretStore) Set(id string, secret string) error {
	if err, ok := m.setErr[id]; ok {
		return err
	}
	m.secrets[id] = secret
	return nil
}

func (m *memorySecretStore) Get(id string) (string, error) {
	if err, ok := m.getErr[id]; ok {
		return "", err
	}
	secret, ok := m.secrets[id]
	if !ok {
		return "", errSecretNotFound
	}
	return secret, nil
}

func (m *memorySecretStore) Delete(id string) error {
	delete(m.secrets, id)
	return nil
}

func TestResolveTranslatorSecretReadsFromStore(t *testing.T) {
	store := newMemorySecretStore()
	store.secrets["BaiduTranslator"] = "stored-secret"

	secret, err := resolveTranslatorSecret("BaiduTranslator", store)
	if err != nil {
		t.Fatalf("应从系统密钥存储读取成功: %v", err)
	}
	if secret != "stored-secret" {
		t.Fatalf("读取结果错误: secret=%q", secret)
	}
}

func TestResolveTranslatorSecretReturnsNotFound(t *testing.T) {
	store := newMemorySecretStore()

	_, err := resolveTranslatorSecret("BaiduTranslator", store)
	if !errors.Is(err, errSecretNotFound) {
		t.Fatalf("未命中时应返回 errSecretNotFound，got %v", err)
	}
}

func TestResolveTranslatorSecretReturnsStoreError(t *testing.T) {
	store := newMemorySecretStore()
	store.getErr["BaiduTranslator"] = errors.New("boom")

	_, err := resolveTranslatorSecret("BaiduTranslator", store)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("应直接返回存储层错误，got %v", err)
	}
}

func TestBuildTranslatorInitDataStoresSecretsInKeyring(t *testing.T) {
	store := newMemorySecretStore()
	b := &fakeTranslator{id: "BaiduTranslator", name: "百度翻译", secret: "appid key", enabled: true}

	items, err := buildTranslatorInitData([]translator.Translator{b}, store)
	if err != nil {
		t.Fatalf("保存到系统密钥存储不应报错: %v", err)
	}
	if len(items) != 1 || items[0].Id != b.Id() {
		t.Fatalf("配置中应只保留 translator id，got %#v", items)
	}
	if got := store.secrets[b.Id()]; got != "appid key" {
		t.Fatalf("系统密钥存储内容错误: %q", got)
	}
}

func TestBuildTranslatorInitDataReturnsStoreError(t *testing.T) {
	store := newMemorySecretStore()
	b := &fakeTranslator{id: "BaiduTranslator", name: "百度翻译", secret: "appid key", enabled: true}
	store.setErr[b.Id()] = errors.New("save failed")

	_, err := buildTranslatorInitData([]translator.Translator{b}, store)
	if err == nil || err.Error() != "save failed" {
		t.Fatalf("保存失败时应直接返回错误，got %v", err)
	}
}
