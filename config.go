package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HistoryGroupData struct {
	Active  bool        `json:"active"`
	History []*ClipItem `json:"history"`
}

type HistoryData struct {
	History    []*ClipItem                 `json:"history"`
	Groups     map[string]HistoryGroupData `json:"groups"`
	GroupNames []string                    `json:"group_names"`
}

type TranslatorInitData struct {
	Id string `json:"id"`
}

type TranslatorData struct {
	Lang                string               `json:"lang"`
	CurrentTranslatorId *string              `json:"current_translator_id"`
	InitedTranslators   []TranslatorInitData `json:"inited_translators"`
}

type Config struct {
	HistoryMax         uint            `json:"history_max"`
	SingleDelete       bool            `json:"single_delete"`
	AutoRecognizeColor bool            `json:"auto_recognize_color"`
	SaveLogToLocal     bool            `json:"save_log_to_local"`
	Data               HistoryData     `json:"data"`
	Translator         *TranslatorData `json:"translator_data"`
}

func NewDefaultConfig() *Config {
	return &Config{
		HistoryMax:         50,
		SingleDelete:       false,
		AutoRecognizeColor: false,
		SaveLogToLocal:     false,
		Data: HistoryData{
			History:    nil,
			Groups:     make(map[string]HistoryGroupData),
			GroupNames: []string{},
		},
	}
}

func loadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := NewDefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	if config.Data.Groups == nil {
		config.Data.Groups = make(map[string]HistoryGroupData)
	}
	if config.Data.GroupNames == nil {
		config.Data.GroupNames = []string{}
	}

	return config, nil
}

func uniqueItems(primary []*ClipItem, secondary []*ClipItem) []*ClipItem {
	seen := make(map[string]bool)
	result := make([]*ClipItem, 0, len(primary)+len(secondary))

	appendUnique := func(items []*ClipItem) {
		for _, item := range items {
			if item == nil {
				continue
			}
			key := fmt.Sprintf("%d:%s", item.Type, item.Hash)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, item)
		}
	}

	appendUnique(primary)
	appendUnique(secondary)
	return result
}

func uniqueStrings(primary []string, secondary []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(primary)+len(secondary))

	appendUnique := func(items []string) {
		for _, item := range items {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			result = append(result, item)
		}
	}

	appendUnique(primary)
	appendUnique(secondary)
	return result
}

func mergeConfigs(primary *Config, secondary *Config) *Config {
	if primary == nil && secondary == nil {
		return NewDefaultConfig()
	}
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}

	merged := NewDefaultConfig()
	*merged = *primary
	merged.Data = HistoryData{
		History:    uniqueItems(primary.Data.History, secondary.Data.History),
		Groups:     make(map[string]HistoryGroupData),
		GroupNames: uniqueStrings(primary.Data.GroupNames, secondary.Data.GroupNames),
	}

	allGroupNames := uniqueStrings(primary.Data.GroupNames, secondary.Data.GroupNames)
	groupNameSet := make(map[string]bool)
	for _, name := range allGroupNames {
		groupNameSet[name] = true
	}
	for name := range primary.Data.Groups {
		groupNameSet[name] = true
	}
	for name := range secondary.Data.Groups {
		groupNameSet[name] = true
	}

	groupNames := make([]string, 0, len(groupNameSet))
	for name := range groupNameSet {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for _, name := range groupNames {
		primaryGroup, primaryOK := primary.Data.Groups[name]
		secondaryGroup, secondaryOK := secondary.Data.Groups[name]

		switch {
		case primaryOK && secondaryOK:
			merged.Data.Groups[name] = HistoryGroupData{
				Active:  primaryGroup.Active || secondaryGroup.Active,
				History: uniqueItems(primaryGroup.History, secondaryGroup.History),
			}
		case primaryOK:
			merged.Data.Groups[name] = primaryGroup
		case secondaryOK:
			merged.Data.Groups[name] = secondaryGroup
		}
	}

	merged.Data.GroupNames = uniqueStrings(merged.Data.GroupNames, groupNames)
	return merged
}

func backupLegacyConfig(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	backupPath := path + ".migrated.bak"
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, data, 0644)
}

func saveConfigToPath(path string, config *Config) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func loadMergedConfig() (*Config, string, error) {
	newPath := getConfigPath()
	legacyPath := getLegacyConfigPath()

	newConfig, newErr := loadConfigFromPath(newPath)
	legacyConfig, legacyErr := loadConfigFromPath(legacyPath)

	newExists := newErr == nil
	legacyExists := legacyErr == nil && !samePath(newPath, legacyPath)

	switch {
	case newExists && legacyExists:
		merged := mergeConfigs(newConfig, legacyConfig)
		if err := saveConfigToPath(newPath, merged); err != nil {
			return nil, "", err
		}
		if err := backupLegacyConfig(legacyPath); err == nil {
			_ = os.Remove(legacyPath)
		}
		return merged, "merged", nil
	case newExists:
		return newConfig, "current", nil
	case legacyExists:
		if err := saveConfigToPath(newPath, legacyConfig); err != nil {
			return nil, "", err
		}
		if err := backupLegacyConfig(legacyPath); err == nil {
			_ = os.Remove(legacyPath)
		}
		return legacyConfig, "migrated", nil
	default:
		if newErr != nil && !os.IsNotExist(newErr) {
			return nil, "", newErr
		}
		if legacyErr != nil && !os.IsNotExist(legacyErr) {
			return nil, "", legacyErr
		}
		return NewDefaultConfig(), "default", nil
	}
}

func samePath(a string, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if cleanA == cleanB {
		return true
	}
	return strings.EqualFold(cleanA, cleanB)
}
