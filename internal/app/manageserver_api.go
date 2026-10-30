package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

/* ---------- 视图结构 ---------- */

type manageColorView struct {
	Hex string `json:"hex"`
	RGB string `json:"rgb"`
	R   string `json:"r"`
	G   string `json:"g"`
	B   string `json:"b"`
}

type manageBaseItemView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type manageItemView struct {
	Ref     string               `json:"ref"`
	Type    ItemType             `json:"type"`
	Hash    string               `json:"hash"`
	Name    string               `json:"name,omitempty"`
	Time    time.Time            `json:"time"`
	From    ItemFrom             `json:"from"`
	Content string               `json:"content,omitempty"`
	Color   *manageColorView     `json:"color,omitempty"`
	Bases   []manageBaseItemView `json:"bases,omitempty"`
}

type manageGroupView struct {
	Name   string            `json:"name"`
	Active bool              `json:"active"`
	Items  []*manageItemView `json:"items"`
}

type manageConfigView struct {
	HistoryMax          uint   `json:"historyMax"`
	KeepLatestOnly      bool   `json:"keepLatestOnly"`
	ConfirmClearHistory bool   `json:"confirmClearHistory"`
	SingleDelete        bool   `json:"singleDelete"`
	SearchMenuTop       bool   `json:"searchMenuTop"`
	ImageRename         bool   `json:"imageRename"`
	AddToGroup          bool   `json:"addToGroup"`
	AutoRecognizeColor  bool   `json:"autoRecognizeColor"`
	BaseConvert         bool   `json:"baseConvert"`
	SaveLogToLocal      bool   `json:"saveLogToLocal"`
	HighlightPrefix     string `json:"highlightPrefix"`
	HighlightSuffix     string `json:"highlightSuffix"`
	TimeFormat          string `json:"timeFormat"`
	AutoStart           bool   `json:"autoStart"`
}

// manageConfigPatch 是配置更新请求，只应用出现的字段。
type manageConfigPatch struct {
	HistoryMax          *uint   `json:"historyMax"`
	KeepLatestOnly      *bool   `json:"keepLatestOnly"`
	ConfirmClearHistory *bool   `json:"confirmClearHistory"`
	SingleDelete        *bool   `json:"singleDelete"`
	SearchMenuTop       *bool   `json:"searchMenuTop"`
	ImageRename         *bool   `json:"imageRename"`
	AddToGroup          *bool   `json:"addToGroup"`
	AutoRecognizeColor  *bool   `json:"autoRecognizeColor"`
	BaseConvert         *bool   `json:"baseConvert"`
	SaveLogToLocal      *bool   `json:"saveLogToLocal"`
	HighlightPrefix     *string `json:"highlightPrefix"`
	HighlightSuffix     *string `json:"highlightSuffix"`
	TimeFormat          *string `json:"timeFormat"`
	AutoStart           *bool   `json:"autoStart"`
}

/* ---------- 通用辅助 ---------- */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "缺少请求体")
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
		return false
	}
	return true
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return false
	}
	return true
}

// manageItemViewOf 构建条目视图；按当前配置附带颜色识别与进制转换结果。
func (a *App) manageItemViewOf(item *ClipItem) *manageItemView {
	if item == nil {
		return nil
	}
	view := &manageItemView{
		Ref:  item.Ref(),
		Type: item.Type,
		Hash: item.Hash,
		Name: item.Name,
		Time: item.Time,
		From: item.From,
	}
	if item.Type != TypeText {
		return view
	}

	view.Content = string(item.Content)

	if a.configAutoRecognizeColor.Load() {
		if hex, rgb, r, g, b, ok := colorValuesOf(view.Content); ok {
			view.Color = &manageColorView{Hex: hex, RGB: rgb, R: r, G: g, B: b}
		}
	}

	if a.configBaseConvert.Load() {
		for _, c := range baseConvertItemsOf(view.Content) {
			view.Bases = append(view.Bases, manageBaseItemView{Name: c.name, Value: c.value})
		}
	}

	return view
}

func (a *App) buildManageConfigView() *manageConfigView {
	return &manageConfigView{
		HistoryMax:          uint(a.configHistoryMax.Load()),
		KeepLatestOnly:      a.configKeepLatestOnly.Load(),
		ConfirmClearHistory: a.configConfirmClearHistory.Load(),
		SingleDelete:        a.configSingleDelete.Load(),
		SearchMenuTop:       a.configSearchMenuTop.Load(),
		ImageRename:         a.configImageRename.Load(),
		AddToGroup:          a.configAddToGroup.Load(),
		AutoRecognizeColor:  a.configAutoRecognizeColor.Load(),
		BaseConvert:         a.configBaseConvert.Load(),
		SaveLogToLocal:      a.configSaveLogToLocal.Load(),
		HighlightPrefix:     a.configHighlightPrefix.Load(),
		HighlightSuffix:     a.configHighlightSuffix.Load(),
		TimeFormat:          a.configTimeFormat.Load(),
		AutoStart:           autostartEnabled(),
	}
}

/* ---------- 状态读取 ---------- */

func (s *ManageServer) handleState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	history := make([]*manageItemView, 0)
	for _, item := range s.app.history.GetAll() {
		if view := s.app.manageItemViewOf(item); view != nil {
			history = append(history, view)
		}
	}

	s.app.groupsMu.RLock()
	names := append([]string(nil), s.app.groupNames...)
	groups := make([]*manageGroupView, 0, len(names))
	for _, name := range names {
		group := s.app.groups[name]
		if group == nil {
			continue
		}
		view := &manageGroupView{
			Name:   name,
			Active: group.Active,
			Items:  make([]*manageItemView, 0),
		}
		for _, item := range group.History.GetAll() {
			if itemView := s.app.manageItemViewOf(item); itemView != nil {
				view.Items = append(view.Items, itemView)
			}
		}
		groups = append(groups, view)
	}
	s.app.groupsMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.app.dataVersion.Load(),
		"history": history,
		"groups":  groups,
		"config":  s.app.buildManageConfigView(),
	})
}

func (s *ManageServer) handleImage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	item := s.app.history.FindByRef(r.URL.Query().Get("ref"))
	if item == nil || item.Type != TypeImage {
		writeError(w, http.StatusNotFound, "图片不存在")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=3600")
	_, _ = w.Write(item.Content)
}

/* ---------- 条目操作 ---------- */

// resolveManageHistory 解析条目操作对应的历史：group 为空表示主历史，否则为指定分组。
func (s *ManageServer) resolveManageHistory(group string) (*History, bool) {
	if group == "" {
		return s.app.history, true
	}

	s.app.groupsMu.RLock()
	target := s.app.groups[group]
	s.app.groupsMu.RUnlock()
	if target == nil {
		return nil, false
	}
	return target.History, true
}

func (s *ManageServer) handleItemCopy(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Ref   string `json:"ref"`
		Group string `json:"group"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	history, ok := s.resolveManageHistory(req.Group)
	if !ok {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	item := history.FindByRef(req.Ref)
	if item == nil {
		writeError(w, http.StatusNotFound, "条目不存在或已被删除")
		return
	}

	select {
	case s.app.writer <- item:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusServiceUnavailable, "剪贴板忙碌，请稍后重试")
	}
}

func (s *ManageServer) handleItemDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Ref   string `json:"ref"`
		Group string `json:"group"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	history, ok := s.resolveManageHistory(req.Group)
	if !ok {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	if !history.DeleteByRef(req.Ref) {
		writeError(w, http.StatusNotFound, "条目不存在或已被删除")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ManageServer) handleItemRename(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Ref   string `json:"ref"`
		Name  string `json:"name"`
		Group string `json:"group"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	history, ok := s.resolveManageHistory(req.Group)
	if !ok {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	item := history.FindByRef(req.Ref)
	if item == nil {
		writeError(w, http.StatusNotFound, "条目不存在或已被删除")
		return
	}
	if item.Type != TypeImage {
		writeError(w, http.StatusBadRequest, "仅图片条目支持重命名")
		return
	}

	name := limitRunes(strings.TrimSpace(req.Name), maxImageNameRunes)
	if name == "" {
		writeError(w, http.StatusBadRequest, "名称不能为空")
		return
	}
	if !history.RenameByRef(req.Ref, name) {
		writeError(w, http.StatusNotFound, "条目不存在或已被删除")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ManageServer) handleItemAddToGroup(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Ref    string `json:"ref"`
		Group  string `json:"group"`  // 条目所在列表（空=主历史）
		Target string `json:"target"` // 目标分组
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	source, ok := s.resolveManageHistory(req.Group)
	if !ok {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	item := source.FindByRef(req.Ref)
	if item == nil {
		writeError(w, http.StatusNotFound, "条目不存在或已被删除")
		return
	}

	s.app.groupsMu.RLock()
	group := s.app.groups[req.Target]
	s.app.groupsMu.RUnlock()
	if group == nil {
		writeError(w, http.StatusNotFound, "目标分组不存在")
		return
	}

	s.app.addItemToHistory(group.History, item.Clone())
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理添加条目到分组 %s", req.Target)}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleClipboardText 将指定文本写入系统剪贴板（供复制转换结果等场景使用）。
func (s *ManageServer) handleClipboardText(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "内容不能为空")
		return
	}

	select {
	case s.app.writer <- NewClipItem(TypeText, []byte(req.Text)):
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusServiceUnavailable, "剪贴板忙碌，请稍后重试")
	}
}

func (s *ManageServer) handleHistoryClear(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	s.app.history.Clear()
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "网页管理清空历史记录"}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

/* ---------- 分组操作 ---------- */

func (s *ManageServer) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || strings.ContainsAny(name, "\n\r") {
		writeError(w, http.StatusBadRequest, "分组名不能为空且不能包含换行")
		return
	}

	app := s.app
	app.groupsMu.Lock()
	if _, exists := app.groups[name]; exists {
		app.groupsMu.Unlock()
		writeError(w, http.StatusConflict, "分组已存在")
		return
	}
	group := NewGroup(name, false, const_max_history)
	group.History.SetOnChange(app.requestSave)
	app.groups[name] = group
	app.groupNames = append(app.groupNames, name)
	app.groupsMu.Unlock()

	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理创建分组: %s", name)}
	app.requestSave()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ManageServer) handleGroupRename(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Name    string `json:"name"`
		NewName string `json:"newName"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	newName := strings.TrimSpace(req.NewName)
	if newName == "" || strings.ContainsAny(newName, "\n\r") {
		writeError(w, http.StatusBadRequest, "分组名不能为空且不能包含换行")
		return
	}

	app := s.app
	app.groupsMu.RLock()
	_, exists := app.groups[req.Name]
	_, conflict := app.groups[newName]
	app.groupsMu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}
	if conflict && newName != req.Name {
		writeError(w, http.StatusConflict, "新分组名已存在")
		return
	}
	if newName == req.Name {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	if !app.renameGroup(req.Name, newName) {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理重命名分组: %s -> %s", req.Name, newName)}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ManageServer) handleGroupDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if !s.app.deleteGroup(req.Name) {
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}

	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理删除分组: %s", req.Name)}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ManageServer) handleGroupToggle(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	app := s.app
	app.groupsMu.Lock()
	group := app.groups[req.Name]
	if group == nil {
		app.groupsMu.Unlock()
		writeError(w, http.StatusNotFound, "分组不存在")
		return
	}
	group.Active = !group.Active
	active := group.Active
	app.groupsMu.Unlock()

	if active && syncLatestHistoryToGroup(app.history, group) {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理同步最近一条历史记录到分组 %s", group.Name)}
	}
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理%s分组: %s", Ifel(active, "激活", "取消激活"), group.Name)}
	app.requestSave()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": active})
}

/* ---------- 配置 ---------- */

func (s *ManageServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var patch manageConfigPatch
	if !decodeJSONBody(w, r, &patch) {
		return
	}

	if err := s.app.applyManageConfig(&patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// applyManageConfig 应用网页管理提交的配置补丁；只更新出现的字段。
func (a *App) applyManageConfig(p *manageConfigPatch) error {
	if p.HistoryMax != nil {
		v := *p.HistoryMax
		if v < 1 || v > const_max_history {
			return fmt.Errorf("最大历史记录条数需在 1-%d 之间", const_max_history)
		}
		a.configHistoryMax.Store(uint32(v))
		a.history.SetMaxSize(uint(v))
	}

	if p.KeepLatestOnly != nil && *p.KeepLatestOnly != a.configKeepLatestOnly.Load() {
		a.configKeepLatestOnly.Store(*p.KeepLatestOnly)
		if *p.KeepLatestOnly {
			removed := a.history.RemoveDuplicates()
			a.groupsMu.RLock()
			for _, group := range a.groups {
				removed += group.History.RemoveDuplicates()
			}
			a.groupsMu.RUnlock()
			global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("相同内容只保留最新已开启，清理了 %d 条重复记录", removed)}
		}
	}

	if p.ConfirmClearHistory != nil {
		a.configConfirmClearHistory.Store(*p.ConfirmClearHistory)
	}
	if p.SingleDelete != nil {
		a.configSingleDelete.Store(*p.SingleDelete)
	}
	if p.SearchMenuTop != nil {
		a.configSearchMenuTop.Store(*p.SearchMenuTop)
	}
	if p.ImageRename != nil {
		a.configImageRename.Store(*p.ImageRename)
	}
	if p.AddToGroup != nil {
		a.configAddToGroup.Store(*p.AddToGroup)
	}
	if p.AutoRecognizeColor != nil {
		a.configAutoRecognizeColor.Store(*p.AutoRecognizeColor)
	}
	if p.BaseConvert != nil {
		a.configBaseConvert.Store(*p.BaseConvert)
	}
	if p.SaveLogToLocal != nil {
		a.configSaveLogToLocal.Store(*p.SaveLogToLocal)
	}

	if p.HighlightPrefix != nil {
		if *p.HighlightPrefix == "" {
			return fmt.Errorf("搜索高亮前缀不能为空")
		}
		a.configHighlightPrefix.Store(*p.HighlightPrefix)
	}
	if p.HighlightSuffix != nil {
		if *p.HighlightSuffix == "" {
			return fmt.Errorf("搜索高亮后缀不能为空")
		}
		a.configHighlightSuffix.Store(*p.HighlightSuffix)
	}
	if p.TimeFormat != nil {
		if *p.TimeFormat == "" {
			return fmt.Errorf("时间格式不能为空")
		}
		a.configTimeFormat.Store(*p.TimeFormat)
	}

	if p.AutoStart != nil {
		if err := a.setAutostart(*p.AutoStart); err != nil {
			return fmt.Errorf("设置开机自启动失败: %v", err)
		}
	}

	global_log_channel <- LogEntry{Kind: KindInfo, Content: "网页管理更新配置"}
	a.requestSave()
	return nil
}
