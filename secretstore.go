package main

import (
	"errors"

	"clip/translator"
	keyring "github.com/zalando/go-keyring"
)

const secretStoreService = "clip"

var errSecretNotFound = errors.New("secret not found")

type SecretStore interface {
	Set(id string, secret string) error
	Get(id string) (string, error)
	Delete(id string) error
}

type keyringSecretStore struct {
	service string
}

var translatorSecretStore SecretStore = keyringSecretStore{service: secretStoreService}

func (s keyringSecretStore) Set(id string, secret string) error {
	return keyring.Set(s.service, secretStoreAccount(id), secret)
}

func (s keyringSecretStore) Get(id string) (string, error) {
	secret, err := keyring.Get(s.service, secretStoreAccount(id))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errSecretNotFound
	}
	return secret, err
}

func (s keyringSecretStore) Delete(id string) error {
	err := keyring.Delete(s.service, secretStoreAccount(id))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func secretStoreAccount(id string) string {
	return "translator:" + id
}

func resolveTranslatorSecret(id string, store SecretStore) (string, error) {
	if store == nil {
		return "", errSecretNotFound
	}
	return store.Get(id)
}

func buildTranslatorInitData(translators []translator.Translator, store SecretStore) ([]TranslatorInitData, error) {
	if store == nil {
		return nil, errSecretNotFound
	}

	result := make([]TranslatorInitData, 0, len(translators))
	for _, item := range translators {
		if !item.IsEnabled() {
			continue
		}
		if err := store.Set(item.Id(), item.Secret()); err != nil {
			return nil, err
		}
		result = append(result, TranslatorInitData{Id: item.Id()})
	}

	return result, nil
}
