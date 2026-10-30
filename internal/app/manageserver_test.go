package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestManageServer 启动一个测试用网页管理服务，测试结束自动停止。
func newTestManageServer(t *testing.T) (*App, *ManageServer) {
	t.Helper()
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	server, err := NewManageServer(app)
	if err != nil {
		t.Fatalf("构造网页管理失败: %v", err)
	}
	server.Start()
	t.Cleanup(server.Stop)
	return app, server
}

func managePost(t *testing.T, server *ManageServer, path string, body any) (*http.Response, string) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}
	resp, err := http.Post(server.URL()+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func TestManageServerLifecycleAndState(t *testing.T) {
	app, server := newTestManageServer(t)
	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("hello-manage")))

	resp, err := http.Get(server.URL() + "/")
	if err != nil {
		t.Fatalf("获取首页失败: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(page, []byte("Clip")) {
		t.Fatalf("首页异常: status=%d", resp.StatusCode)
	}

	for _, asset := range []string{"/style.css", "/app.js"} {
		assetResp, err := http.Get(server.URL() + asset)
		if err != nil {
			t.Fatalf("获取静态资源 %s 失败: %v", asset, err)
		}
		assetResp.Body.Close()
		if assetResp.StatusCode != http.StatusOK {
			t.Fatalf("静态资源 %s 缺失: %d", asset, assetResp.StatusCode)
		}
	}

	resp, err = http.Get(server.URL() + "/api/state")
	if err != nil {
		t.Fatalf("获取状态失败: %v", err)
	}
	var state struct {
		Version uint64 `json:"version"`
		History []struct {
			Ref     string `json:"ref"`
			Type    int    `json:"type"`
			Content string `json:"content"`
		} `json:"history"`
		Groups []any          `json:"groups"`
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("解析状态失败: %v", err)
	}
	resp.Body.Close()

	if len(state.History) != 1 || state.History[0].Content != "hello-manage" {
		t.Fatalf("历史状态异常: %+v", state.History)
	}
	if state.History[0].Ref == "" {
		t.Fatalf("条目应携带引用")
	}
	if state.Config["historyMax"] == nil {
		t.Fatalf("配置应包含 historyMax")
	}

	server.Stop()
	server.Stop() // 重复停止应安全

	if _, err := http.Get(server.URL() + "/api/state"); err == nil {
		t.Fatalf("服务停止后请求应失败")
	}
}

func TestManageItemAPI(t *testing.T) {
	app, server := newTestManageServer(t)
	app.writer = make(chan *ClipItem, 4)

	item := NewClipItem(TypeText, []byte("copy-target"))
	app.addItemToHistory(app.history, item)

	resp, _ := managePost(t, server, "/api/items/copy", map[string]string{"ref": item.Ref()})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("复制应成功: %d", resp.StatusCode)
	}
	select {
	case got := <-app.writer:
		if string(got.Content) != "copy-target" {
			t.Fatalf("剪贴板写入内容错误: %q", string(got.Content))
		}
	default:
		t.Fatalf("剪贴板写入通道应收到条目")
	}

	resp, _ = managePost(t, server, "/api/items/delete", map[string]string{"ref": item.Ref()})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("删除应成功: %d", resp.StatusCode)
	}
	if app.history.FindByRef(item.Ref()) != nil {
		t.Fatalf("条目应被删除")
	}

	resp, _ = managePost(t, server, "/api/items/delete", map[string]string{"ref": "0:missing:1"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("删除不存在条目应返回 404: %d", resp.StatusCode)
	}
}

func TestManageItemRenameAndImage(t *testing.T) {
	app, server := newTestManageServer(t)

	image := NewClipItem(TypeImage, []byte("fake-png-bytes"))
	app.addItemToHistory(app.history, image)

	resp, _ := managePost(t, server, "/api/items/rename", map[string]string{"ref": image.Ref(), "name": "截图"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("图片重命名应成功: %d", resp.StatusCode)
	}
	renamed := app.history.FindByRef(image.Ref())
	if renamed == nil || renamed.Name != "截图" {
		t.Fatalf("重命名未生效: %#v", renamed)
	}

	text := NewClipItem(TypeText, []byte("plain-text"))
	app.addItemToHistory(app.history, text)
	resp, _ = managePost(t, server, "/api/items/rename", map[string]string{"ref": text.Ref(), "name": "x"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("文本条目重命名应被拒绝: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/items/rename", map[string]string{"ref": image.Ref(), "name": "  "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("空名称应被拒绝: %d", resp.StatusCode)
	}

	imgResp, err := http.Get(server.URL() + "/api/image?ref=" + url.QueryEscape(image.Ref()))
	if err != nil {
		t.Fatalf("获取图片失败: %v", err)
	}
	body, _ := io.ReadAll(imgResp.Body)
	imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK || imgResp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("图片响应异常: status=%d type=%s", imgResp.StatusCode, imgResp.Header.Get("Content-Type"))
	}
	if string(body) != "fake-png-bytes" {
		t.Fatalf("图片内容错误: %q", string(body))
	}

	missingResp, err := http.Get(server.URL() + "/api/image?ref=" + url.QueryEscape(text.Ref()))
	if err != nil {
		t.Fatalf("获取图片失败: %v", err)
	}
	missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("文本条目请求图片应 404: %d", missingResp.StatusCode)
	}
}

func TestManageAddToGroup(t *testing.T) {
	app, server := newTestManageServer(t)
	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("group-target")))

	app.groupsMu.Lock()
	group := NewGroup("工作", false, const_max_history)
	app.groups["工作"] = group
	app.groupNames = append(app.groupNames, "工作")
	app.groupsMu.Unlock()

	item := app.history.GetTop()
	resp, _ := managePost(t, server, "/api/items/add-to-group", map[string]string{"ref": item.Ref(), "target": "工作"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("添加到分组应成功: %d", resp.StatusCode)
	}
	if group.History.GetTop() == nil {
		t.Fatalf("分组应收到条目副本")
	}
	if group.History.GetTop() == item {
		t.Fatalf("分组条目应为克隆而非原指针")
	}

	resp, _ = managePost(t, server, "/api/items/add-to-group", map[string]string{"ref": item.Ref(), "target": "不存在"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("添加到不存在分组应 404: %d", resp.StatusCode)
	}
}

func TestManageItemOperationsInGroup(t *testing.T) {
	app, server := newTestManageServer(t)

	app.groupsMu.Lock()
	group := NewGroup("工作", false, const_max_history)
	app.groups["工作"] = group
	app.groupNames = append(app.groupNames, "工作")
	app.groupsMu.Unlock()

	// 分组条目与主历史中的同内容条目共享相同的 ref
	groupItem := NewClipItem(TypeText, []byte("in-group"))
	group.History.Add(groupItem)
	mainItem := groupItem.Clone()
	app.addItemToHistory(app.history, mainItem)

	// 在分组中删除：只删分组条目，主历史不受影响
	resp, _ := managePost(t, server, "/api/items/delete", map[string]string{"ref": groupItem.Ref(), "group": "工作"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("分组中删除应成功: %d", resp.StatusCode)
	}
	if group.History.FindByRef(groupItem.Ref()) != nil {
		t.Fatalf("分组条目应被删除")
	}
	if app.history.FindByRef(mainItem.Ref()) == nil {
		t.Fatalf("主历史同 ref 条目不应被误删")
	}

	// 再次删除同一引用应 404
	resp, _ = managePost(t, server, "/api/items/delete", map[string]string{"ref": groupItem.Ref(), "group": "工作"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("重复删除分组条目应 404: %d", resp.StatusCode)
	}

	// 分组不存在
	resp, _ = managePost(t, server, "/api/items/delete", map[string]string{"ref": "0:x:1", "group": "不存在"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("操作不存在分组应 404: %d", resp.StatusCode)
	}

	// 分组中重命名图片
	image := NewClipItem(TypeImage, []byte("group-img-bytes"))
	group.History.Add(image)
	resp, _ = managePost(t, server, "/api/items/rename", map[string]string{"ref": image.Ref(), "name": "分组截图", "group": "工作"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("分组中重命名应成功: %d", resp.StatusCode)
	}
	renamed := group.History.FindByRef(image.Ref())
	if renamed == nil || renamed.Name != "分组截图" {
		t.Fatalf("分组重命名未生效: %#v", renamed)
	}

	// 分组间转移：源 group=工作，目标 target=个人
	app.groupsMu.Lock()
	personal := NewGroup("个人", false, const_max_history)
	app.groups["个人"] = personal
	app.groupNames = append(app.groupNames, "个人")
	app.groupsMu.Unlock()

	src := NewClipItem(TypeText, []byte("move-me"))
	group.History.Add(src)
	resp, _ = managePost(t, server, "/api/items/add-to-group", map[string]string{"ref": src.Ref(), "group": "工作", "target": "个人"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("分组间转移应成功: %d", resp.StatusCode)
	}
	if personal.History.GetTop() == nil {
		t.Fatalf("目标分组应收到条目副本")
	}
}

func TestManageGroupAPI(t *testing.T) {
	app, server := newTestManageServer(t)

	resp, _ := managePost(t, server, "/api/groups/create", map[string]string{"name": "工作"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("创建分组应成功: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/groups/create", map[string]string{"name": "工作"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重复创建应 409: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/groups/create", map[string]string{"name": "  "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("空分组名应 400: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/groups/rename", map[string]string{"name": "工作", "newName": "项目"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重命名分组应成功: %d", resp.StatusCode)
	}
	app.groupsMu.RLock()
	_, renamed := app.groups["项目"]
	_, oldGone := app.groups["工作"]
	app.groupsMu.RUnlock()
	if !renamed || oldGone {
		t.Fatalf("分组重命名未生效")
	}

	resp, _ = managePost(t, server, "/api/groups/toggle", map[string]string{"name": "项目"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("切换分组应成功: %d", resp.StatusCode)
	}
	app.groupsMu.RLock()
	active := app.groups["项目"].Active
	app.groupsMu.RUnlock()
	if !active {
		t.Fatalf("分组应被激活")
	}

	resp, _ = managePost(t, server, "/api/groups/delete", map[string]string{"name": "项目"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("删除分组应成功: %d", resp.StatusCode)
	}
	app.groupsMu.RLock()
	_, stillThere := app.groups["项目"]
	app.groupsMu.RUnlock()
	if stillThere {
		t.Fatalf("分组应被删除")
	}

	resp, _ = managePost(t, server, "/api/groups/delete", map[string]string{"name": "不存在"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("删除不存在分组应 404: %d", resp.StatusCode)
	}
}

func TestManageConfigAPI(t *testing.T) {
	app, server := newTestManageServer(t)

	resp, _ := managePost(t, server, "/api/config", map[string]any{"historyMax": 120, "baseConvert": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("更新配置应成功: %d", resp.StatusCode)
	}
	if app.configHistoryMax.Load() != 120 || !app.configBaseConvert.Load() {
		t.Fatalf("配置未生效: max=%d base=%v", app.configHistoryMax.Load(), app.configBaseConvert.Load())
	}

	resp, _ = managePost(t, server, "/api/config", map[string]any{"historyMax": 0})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("越界条数应 400: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/config", map[string]any{"highlightPrefix": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("空高亮前缀应 400: %d", resp.StatusCode)
	}

	resp, _ = managePost(t, server, "/api/config", map[string]any{"timeFormat": "YYYY-MM-DD HH:mm"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("更新时间格式应成功: %d", resp.StatusCode)
	}
	if app.configTimeFormat.Load() != "YYYY-MM-DD HH:mm" {
		t.Fatalf("时间格式未更新: %s", app.configTimeFormat.Load())
	}
}

func TestManageStateItemConversions(t *testing.T) {
	app, server := newTestManageServer(t)
	app.configAutoRecognizeColor.Store(true)
	app.configBaseConvert.Store(true)

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("255")))
	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("#aabbcc")))

	resp, err := http.Get(server.URL() + "/api/state")
	if err != nil {
		t.Fatalf("获取状态失败: %v", err)
	}
	defer resp.Body.Close()

	var state struct {
		History []struct {
			Content string `json:"content"`
			Color   *struct {
				Hex string `json:"hex"`
				RGB string `json:"rgb"`
				R   string `json:"r"`
				G   string `json:"g"`
				B   string `json:"b"`
			} `json:"color"`
			Bases []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"bases"`
		} `json:"history"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("解析状态失败: %v", err)
	}
	if len(state.History) != 2 {
		t.Fatalf("历史条目数错误: %d", len(state.History))
	}

	color, number := state.History[0], state.History[1]
	if len(number.Bases) != 3 || number.Bases[0].Name != "二进制" || number.Bases[0].Value != "0b11111111" {
		t.Fatalf("数字条目应带进制转换结果: %+v", number.Bases)
	}
	if number.Color != nil {
		t.Fatalf("纯数字不应识别为颜色")
	}

	if color.Color == nil || color.Color.Hex != "#aabbcc" || color.Color.RGB != "170,187,204" || color.Color.R != "170" {
		t.Fatalf("颜色条目应带颜色解析结果: %+v", color.Color)
	}
	if len(color.Bases) != 0 {
		t.Fatalf("颜色文本不应带进制转换结果")
	}

	app.configAutoRecognizeColor.Store(false)
	app.configBaseConvert.Store(false)
}

func TestManageClipboardText(t *testing.T) {
	app, server := newTestManageServer(t)
	app.writer = make(chan *ClipItem, 4)

	resp, _ := managePost(t, server, "/api/clipboard/text", map[string]string{"text": "0b1010"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("写入剪贴板应成功: %d", resp.StatusCode)
	}
	select {
	case item := <-app.writer:
		if string(item.Content) != "0b1010" {
			t.Fatalf("写入内容错误: %q", string(item.Content))
		}
	default:
		t.Fatalf("剪贴板写入通道应收到内容")
	}

	resp, _ = managePost(t, server, "/api/clipboard/text", map[string]string{"text": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("空文本应 400: %d", resp.StatusCode)
	}
}

func TestManageHostCheck(t *testing.T) {
	_, server := newTestManageServer(t)

	req, _ := http.NewRequest(http.MethodGet, server.URL()+"/api/state", nil)
	req.Host = "evil.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("非法 Host 应拒绝: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, server.URL()+"/api/state", nil)
	req.Host = fmt.Sprintf("localhost:%d", server.port)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost 访问应允许: %d", resp.StatusCode)
	}
}

func TestManageSSEBroadcastsChanges(t *testing.T) {
	app, server := newTestManageServer(t)

	resp, err := http.Get(server.URL() + "/api/events")
	if err != nil {
		t.Fatalf("连接 SSE 失败: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("SSE Content-Type 错误: %s", ct)
	}

	lines := make(chan string, 32)
	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				close(lines)
				return
			}
			lines <- line
		}
	}()

	parseVersion := func(line string) (uint64, bool) {
		payload := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
		var v struct {
			Version uint64 `json:"version"`
		}
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return 0, false
		}
		return v.Version, true
	}

	waitDataLine := func(timeout time.Duration) (uint64, bool) {
		deadline := time.After(timeout)
		for {
			select {
			case <-deadline:
				return 0, false
			case line, ok := <-lines:
				if !ok {
					return 0, false
				}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				if version, ok := parseVersion(line); ok {
					return version, true
				}
			}
		}
	}

	initial, ok := waitDataLine(3 * time.Second)
	if !ok {
		t.Fatalf("应收到初始版本推送")
	}

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("sse-change")))

	changed, ok := waitDataLine(5 * time.Second)
	if !ok {
		t.Fatalf("数据变更后应收到 SSE 推送")
	}
	if changed <= initial {
		t.Fatalf("推送版本应递增: initial=%d changed=%d", initial, changed)
	}
}
