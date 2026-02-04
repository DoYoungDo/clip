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
	HistoryMax uint `json:"history_max"`
	SingleDelete bool `json:"single_delete"`
	AutoRecognizeColor bool `json:"auto_recognize_color"`
	Data HistoryData `json:"data"`
}

func NewDefaultConfig() *Config{
	return &Config{
		HistoryMax: 50,
		SingleDelete: false,
		AutoRecognizeColor: false,
		Data: HistoryData{
			History: nil,
			Groups: make(map[string]HistoryGroupData),
			GroupNames: []string{},
		},
	}
}