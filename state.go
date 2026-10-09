package main

import "clip/translator"

type ClearState int

const (
	Normal ClearState = iota
	ReadyToClear
)

type ShowMenuState int

const (
	Click ShowMenuState = iota
	RClick
)

// 全局状态
var (
	global_clear_state                             = Normal
	global_show_menu_state                         = Click
	global_search_enable                           = false
	global_search_text                             = ""
	global_translate_to_lang translator.TransLang  = translator.ZH
	global_translator        translator.Translator = nil
	global_menu_title        string                = ""
)

// 全局常量
const (
	const_max_history uint = 300
)

// 全局配置
var (
	config_history_max          uint = const_max_history
	config_single_delete             = false
	config_auto_recognize_color      = false
	config_save_log_to_local         = false
)
