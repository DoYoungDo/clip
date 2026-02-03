package main

type HistoryGroupData struct{
	Active bool `json:"active"`
	History []*ClipItem `json:"history"`
}

type HistoryData struct{
	History []*ClipItem `json:"history"`
	Groups map[string]HistoryGroupData `json:"groups"`
	GroupNames []string `json:"group_names"`
}

type Config struct{
	HistoryMax int `json:"history_max"`
	SingleDelete bool `json:"single_delete"`
	Data HistoryData `json:"data"`
}

func NewDefaultConfig() *Config{
	return &Config{
		HistoryMax: 50,
		SingleDelete: false,
		Data: HistoryData{
			History: nil,
			Groups: make(map[string]HistoryGroupData),
			GroupNames: []string{},
		},
	}
}