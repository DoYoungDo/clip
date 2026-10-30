"use strict";

/* ==================== 状态 ==================== */

const state = {
  version: 0,
  history: [],
  groups: [],
  config: {},
  selected: "all",
  keyword: "",
};

let configPanelOpen = false;

/* ==================== 基础工具 ==================== */

const $ = (sel) => document.querySelector(sel);

function escapeHtml(s) {
  return String(s ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function timeText(iso) {
  const d = new Date(iso);
  return isNaN(d) ? "" : d.toLocaleString([], { hour12: false });
}

async function api(path, body) {
  const options = { method: body === undefined ? "GET" : "POST" };
  if (body !== undefined) {
    options.headers = { "Content-Type": "application/json" };
    options.body = JSON.stringify(body);
  }
  const res = await fetch(path, options);
  let data = null;
  try {
    data = await res.json();
  } catch (_) {
    /* 空响应体 */
  }
  if (!res.ok) {
    throw new Error((data && data.error) || "请求失败: " + res.status);
  }
  return data;
}

function toast(message, isError = false) {
  const el = document.createElement("div");
  el.className = "toast" + (isError ? " error" : "");
  el.textContent = message;
  $("#toast-root").appendChild(el);
  setTimeout(() => el.remove(), 2200);
}

async function exec(path, body, successMessage) {
  try {
    await api(path, body);
    if (successMessage) toast(successMessage);
    await refresh();
  } catch (err) {
    toast(err.message, true);
  }
}

/* ==================== 弹层 ==================== */

function showModal({ title, message, input, initial, confirmText = "确定", danger = false, onConfirm }) {
  const modal = $("#modal");
  const mask = $("#modal-mask");
  modal.classList.remove("wide");
  modal.innerHTML = `
    <h3>${escapeHtml(title)}</h3>
    ${message ? `<p>${escapeHtml(message)}</p>` : ""}
    ${input ? `<input id="modal-input" class="rename-input" type="text" value="${escapeHtml(initial || "")}" placeholder="${escapeHtml(input)}">` : ""}
    <div class="modal-actions">
      <button class="btn" id="modal-cancel">取消</button>
      <button class="btn ${danger ? "danger" : "primary"}" id="modal-ok">${escapeHtml(confirmText)}</button>
    </div>`;
  modal.hidden = false;
  mask.hidden = false;

  const inputEl = $("#modal-input");
  if (inputEl) {
    inputEl.focus();
    inputEl.select();
  }

  const close = () => {
    modal.hidden = true;
    mask.hidden = true;
    modal.innerHTML = "";
  };
  $("#modal-cancel").onclick = close;
  mask.onclick = close;
  $("#modal-ok").onclick = async () => {
    const value = inputEl ? inputEl.value.trim() : undefined;
    close();
    try {
      await onConfirm(value);
    } catch (err) {
      toast(err.message, true);
    }
  };
  if (inputEl) {
    inputEl.onkeydown = (e) => {
      if (e.key === "Enter") $("#modal-ok").click();
    };
  }
}

function showItemDetail(item) {
  const modal = $("#modal");
  const mask = $("#modal-mask");
  const meta = [timeText(item.time), item.from === 1 ? "远端" : "本地"];
  if (item.type === 1) {
    meta.push(item.name || "#" + item.hash.slice(0, 8));
  }

  modal.classList.add("wide");
  modal.innerHTML = `
    <h3>历史详情</h3>
    <p>${escapeHtml(meta.join(" · "))}</p>
    <pre class="detail-text">${escapeHtml(item.content || "")}</pre>
    <div class="modal-actions">
      <button class="btn" id="modal-cancel">关闭</button>
      <button class="btn primary" id="modal-ok">复制</button>
    </div>`;
  modal.hidden = false;
  mask.hidden = false;

  const close = () => {
    modal.hidden = true;
    mask.hidden = true;
    modal.classList.remove("wide");
    modal.innerHTML = "";
  };
  $("#modal-cancel").onclick = close;
  mask.onclick = close;
  $("#modal-ok").onclick = async () => {
    try {
      await api("/api/items/copy", { ref: item.ref, group: currentGroupParam() });
      toast("已复制到剪贴板");
    } catch (err) {
      toast(err.message, true);
    }
  };
}

/* ==================== 数据加载与渲染 ==================== */

async function refresh() {
  const data = await api("/api/state");
  state.version = data.version || 0;
  state.history = data.history || [];
  state.groups = data.groups || [];
  state.config = data.config || {};

  if (state.selected !== "all" && !state.groups.some((g) => g.name === state.selected)) {
    state.selected = "all";
  }

  renderNav();
  renderList();
}

function navItemHtml(key, icon, name, count, active, isGroup) {
  const activeCls = state.selected === key ? " active" : "";
  const dot = isGroup
    ? `<span class="nav-dot${active ? " active" : ""}" data-nav-action="toggle" data-name="${escapeHtml(name)}" title="点击激活/取消激活分组"></span>`
    : "";
  const actions = isGroup
    ? `<span class="nav-actions">
         <button data-nav-action="rename-group" data-name="${escapeHtml(name)}" title="重命名">✎</button>
         <button data-nav-action="delete-group" data-name="${escapeHtml(name)}" title="删除">🗑</button>
       </span>`
    : "";
  return `<div class="nav-item${activeCls}" data-select="${escapeHtml(key)}" title="${escapeHtml(key === "all" ? "全部历史" : name)}">
    ${dot}<span>${icon}</span><span class="nav-name">${escapeHtml(name)}</span><span class="nav-count">${count}</span>${actions}
  </div>`;
}

function countMatched(items) {
  return items.filter(matchesKeyword).length;
}

function renderNav() {
  const items = [navItemHtml("all", "📋", "全部历史", countMatched(state.history), false, false)];
  for (const g of state.groups) {
    items.push(navItemHtml(g.name, "📂", g.name, countMatched(g.items), g.active, true));
  }
  $("#nav").innerHTML = items.join("");
}

function currentItems() {
  if (state.selected === "all") return state.history;
  const group = state.groups.find((g) => g.name === state.selected);
  return group ? group.items : [];
}

// currentGroupParam 返回条目所在列表：空串为主历史，否则为当前分组名。
function currentGroupParam() {
  return state.selected === "all" ? "" : state.selected;
}

function matchesKeyword(item) {
  if (!state.keyword) return true;
  const kw = state.keyword.toLowerCase();
  if (item.type === 1) {
    return String(item.name || "").toLowerCase().includes(kw) || item.hash.includes(kw);
  }
  return String(item.content || "").toLowerCase().includes(kw);
}

function describeItem(item) {
  if (!item) return "该条目";
  if (item.type === 1) {
    return item.name ? `图片「${item.name}」` : "该图片";
  }
  const text = String(item.content || "");
  return `「${text.length > 30 ? text.slice(0, 30) + "…" : text}」`;
}

function closeAllDropdowns() {
  document.querySelectorAll(".dropdown-menu").forEach((m) => (m.hidden = true));
  document.querySelectorAll(".card.menu-open").forEach((c) => c.classList.remove("menu-open"));
}

function groupMenuHtml(ref) {
  const targets = state.groups.filter((g) => g.name !== state.selected);
  if (targets.length === 0) {
    return `<div class="menu-empty">暂无其他分组</div>`;
  }
  return targets
    .map((g) => `<button data-action="add-to-group" data-ref="${escapeHtml(ref)}" data-group="${escapeHtml(g.name)}">📂 ${escapeHtml(g.name)}</button>`)
    .join("");
}

function cardHtml(item) {
  const isImage = item.type === 1;
  const ref = escapeHtml(item.ref);
  const thumb = isImage
    ? `<img class="card-thumb" src="/api/image?ref=${encodeURIComponent(item.ref)}" loading="lazy" alt="图片" data-action="view-image" data-ref="${ref}">`
    : `<div class="card-icon">📝</div>`;
  const content = isImage
    ? ""
    : `<div class="card-text" data-action="detail" data-ref="${ref}" title="点击查看完整内容">${escapeHtml(item.content)}</div>`;
  const meta = [`<span>${escapeHtml(timeText(item.time))}</span>`];
  if (item.from === 1) meta.push(`<span class="tag">R 远端</span>`);
  if (isImage) {
    meta.push(`<span>${item.name ? escapeHtml(item.name) : "#" + escapeHtml(item.hash.slice(0, 8))}</span>`);
  }

  // 功能按钮显示与配置项对齐（与托盘菜单行为一致）
  const config = state.config || {};
  const actions = [`<button class="btn primary" data-action="copy" data-ref="${ref}">复制</button>`];
  if (item.color) {
    const c = item.color;
    actions.push(`<div class="dropdown">
        <button class="btn" data-action="toggle-menu"><span class="color-dot" style="background:${escapeHtml(c.hex)}"></span>颜色</button>
        <div class="dropdown-menu" hidden>
          <button data-action="copy-value" data-value="${escapeHtml(c.hex)}">Hex: ${escapeHtml(c.hex)}</button>
          <button data-action="copy-value" data-value="${escapeHtml(c.rgb)}">RGB: ${escapeHtml(c.rgb)}</button>
          <button data-action="copy-value" data-value="${escapeHtml(c.r)}">R: ${escapeHtml(c.r)}</button>
          <button data-action="copy-value" data-value="${escapeHtml(c.g)}">G: ${escapeHtml(c.g)}</button>
          <button data-action="copy-value" data-value="${escapeHtml(c.b)}">B: ${escapeHtml(c.b)}</button>
        </div>
      </div>`);
  }
  if (item.bases && item.bases.length > 0) {
    const rows = item.bases
      .map((c) => `<button data-action="copy-value" data-value="${escapeHtml(c.value)}">${escapeHtml(c.name)}: ${escapeHtml(c.value)}</button>`)
      .join("");
    actions.push(`<div class="dropdown">
        <button class="btn" data-action="toggle-menu">进制转换</button>
        <div class="dropdown-menu" hidden>${rows}</div>
      </div>`);
  }
  if (isImage && config.imageRename) {
    actions.push(`<button class="btn" data-action="rename" data-ref="${ref}">重命名</button>`);
  }
  if (config.addToGroup && state.groups.length > 0) {
    actions.push(`<div class="dropdown">
        <button class="btn" data-action="toggle-menu">添加到分组</button>
        <div class="dropdown-menu" hidden>${groupMenuHtml(item.ref)}</div>
      </div>`);
  }
  if (config.singleDelete) {
    actions.push(`<button class="btn danger" data-action="delete" data-ref="${ref}">删除</button>`);
  }

  return `<div class="card" data-ref="${ref}">
    ${thumb}
    <div class="card-main">
      ${content}
      <div class="card-meta">${meta.join("")}</div>
    </div>
    <div class="card-actions">${actions.join("")}</div>
  </div>`;
}

function renderList() {
  const items = currentItems().filter(matchesKeyword);
  $("#list-title").textContent = state.selected === "all" ? "全部历史" : state.selected;
  $("#list-count").textContent = `${items.length} 条`;
  $("#btn-clear").hidden = state.selected !== "all";
  $("#empty").hidden = items.length > 0;
  $("#empty").textContent = state.keyword ? "没有匹配的记录" : "暂无记录";
  $("#list").innerHTML = items.map(cardHtml).join("");
}

/* ==================== 配置面板 ==================== */

// 配置项顺序与托盘配置菜单保持一致
const configFields = [
  { key: "historyMax", label: "最大历史记录条数", type: "number", min: 1, max: 300 },
  { key: "highlightPrefix", label: "搜索高亮前缀", type: "text" },
  { key: "highlightSuffix", label: "搜索高亮后缀", type: "text" },
  { key: "timeFormat", label: "条目时间格式", type: "text" },
  { key: "confirmClearHistory", label: "清空历史时不询问", type: "bool", invert: true },
  { key: "singleDelete", label: "单独删除项", type: "bool" },
  { key: "autoRecognizeColor", label: "自动识别颜色", type: "bool" },
  { key: "keepLatestOnly", label: "相同内容只保留最新", type: "bool" },
  { key: "imageRename", label: "图片重命名", type: "bool" },
  { key: "searchMenuTop", label: "置顶【搜索】菜单", type: "bool" },
  { key: "addToGroup", label: "添加到分组", type: "bool" },
  { key: "baseConvert", label: "进制转换", type: "bool" },
  { key: "autoStart", label: "开机自启动", type: "bool" },
  { key: "saveLogToLocal", label: "退出时保存日志", type: "bool" },
];

function renderConfig() {
  const cfg = state.config || {};
  let html = "";
  for (const field of configFields) {
    const value = cfg[field.key];
    if (field.type === "bool") {
      const checked = field.invert ? !value : value;
      html += `<div class="config-row">
        <label for="cfg-${field.key}">${escapeHtml(field.label)}</label>
        <span class="switch"><input type="checkbox" id="cfg-${field.key}" data-config="${field.key}" data-invert="${field.invert ? "1" : "0"}" ${checked ? "checked" : ""}><span class="slider"></span></span>
      </div>`;
    } else if (field.type === "number") {
      html += `<div class="config-row">
        <label for="cfg-${field.key}">${escapeHtml(field.label)}</label>
        <input type="number" id="cfg-${field.key}" data-config="${field.key}" value="${Number(value) || 0}" min="${field.min}" max="${field.max}">
      </div>`;
    } else {
      html += `<div class="config-row">
        <label for="cfg-${field.key}">${escapeHtml(field.label)}</label>
        <input type="text" id="cfg-${field.key}" data-config="${field.key}" value="${escapeHtml(value ?? "")}">
      </div>`;
    }
  }
  $("#config-body").innerHTML = html;
}

/* ==================== 主题 ==================== */

const THEME_KEY = "clip-theme";
const THEME_ORDER = ["system", "light", "dark"];
const THEME_META = {
  system: { icon: "💻", label: "跟随系统" },
  light: { icon: "☀️", label: "亮色" },
  dark: { icon: "🌙", label: "暗色" },
};

function currentThemeSetting() {
  return localStorage.getItem(THEME_KEY) || "system";
}

function applyTheme() {
  const setting = currentThemeSetting();
  const dark = setting === "dark" || (setting === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";

  const meta = THEME_META[setting];
  const btn = $("#btn-theme");
  if (btn) {
    btn.textContent = meta.icon;
    btn.title = "主题：" + meta.label + "（点击切换）";
  }
}

function bindThemeToggle() {
  $("#btn-theme").onclick = () => {
    const next = THEME_ORDER[(THEME_ORDER.indexOf(currentThemeSetting()) + 1) % THEME_ORDER.length];
    localStorage.setItem(THEME_KEY, next);
    applyTheme();
  };
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (currentThemeSetting() === "system") applyTheme();
  });
}

/* ==================== 实时事件（SSE） ==================== */

function connectEvents() {
  const conn = $("#conn");
  const es = new EventSource("/api/events");
  es.onopen = () => {
    conn.className = "conn on";
    conn.title = "实时连接正常";
  };
  es.onerror = () => {
    conn.className = "conn off";
    conn.title = "连接断开，自动重连中";
  };
  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      if (typeof data.version === "number" && data.version !== state.version) {
        refresh().catch(() => {});
      }
    } catch (_) {
      /* 忽略非法消息 */
    }
  };
}

/* ==================== 事件绑定 ==================== */

function bindEvents() {
  $("#search").oninput = (e) => {
    state.keyword = e.target.value.trim();
    renderNav();
    renderList();
  };

  $("#nav").addEventListener("click", (e) => {
    const actionBtn = e.target.closest("[data-nav-action]");
    if (actionBtn) {
      e.stopPropagation();
      closeAllDropdowns();
      const name = actionBtn.dataset.name;
      const action = actionBtn.dataset.navAction;
      if (action === "toggle") {
        exec("/api/groups/toggle", { name }, "已切换分组激活状态");
      } else if (action === "rename-group") {
        showModal({
          title: "重命名分组",
          input: "新的分组名",
          initial: name,
          onConfirm: (value) => exec("/api/groups/rename", { name, newName: value }, "分组已重命名"),
        });
      } else if (action === "delete-group") {
        showModal({
          title: "删除分组",
          message: `确定删除分组「${name}」吗？分组内的历史记录将一并删除。`,
          confirmText: "删除",
          danger: true,
          onConfirm: () => exec("/api/groups/delete", { name }, "分组已删除"),
        });
      }
      return;
    }

    const item = e.target.closest(".nav-item");
    if (item) {
      state.selected = item.dataset.select;
      renderNav();
      renderList();
    }
  });

  $("#btn-new-group").onclick = () => {
    showModal({
      title: "新建分组",
      input: "分组名称",
      onConfirm: (value) => exec("/api/groups/create", { name: value }, "分组已创建"),
    });
  };

  $("#btn-clear").onclick = () => {
    showModal({
      title: "清空历史",
      message: "将删除全部历史记录，此操作不可恢复。确定继续吗？",
      confirmText: "清空",
      danger: true,
      onConfirm: () => exec("/api/history/clear", {}, "历史已清空"),
    });
  };

  document.addEventListener("click", async (e) => {
    const menuBtn = e.target.closest('[data-action="toggle-menu"]');
    const inMenu = e.target.closest(".dropdown-menu");

    if (!menuBtn && !inMenu) {
      closeAllDropdowns();
    }

    if (menuBtn) {
      const target = menuBtn.parentElement.querySelector(".dropdown-menu");
      const wasHidden = target.hidden;
      closeAllDropdowns();
      target.hidden = !wasHidden;
      if (!target.hidden) {
        const card = menuBtn.closest(".card");
        if (card) card.classList.add("menu-open");
      }
      return;
    }

    const actionEl = e.target.closest("[data-action]");
    if (!actionEl) return;
    const action = actionEl.dataset.action;
    const ref = actionEl.dataset.ref;

    if (action === "copy") {
      await exec("/api/items/copy", { ref, group: currentGroupParam() }, "已复制到剪贴板");
    } else if (action === "copy-value") {
      await exec("/api/clipboard/text", { text: actionEl.dataset.value }, "已复制: " + actionEl.dataset.value);
    } else if (action === "detail") {
      const item = currentItems().find((x) => x.ref === ref);
      if (item) showItemDetail(item);
    } else if (action === "add-to-group") {
      await exec("/api/items/add-to-group", { ref, group: currentGroupParam(), target: actionEl.dataset.group }, "已添加到分组");
    } else if (action === "rename") {
      const item = currentItems().find((x) => x.ref === ref);
      showModal({
        title: "重命名图片",
        input: "图片名称",
        initial: item && item.name ? item.name : "",
        onConfirm: (value) => exec("/api/items/rename", { ref, name: value, group: currentGroupParam() }, "已重命名"),
      });
    } else if (action === "delete") {
      const item = currentItems().find((x) => x.ref === ref);
      showModal({
        title: "删除条目",
        message: `确定删除${describeItem(item)}吗？此操作不可恢复。`,
        confirmText: "删除",
        danger: true,
        onConfirm: () => exec("/api/items/delete", { ref, group: currentGroupParam() }, "已删除"),
      });
    } else if (action === "view-image") {
      const viewer = document.createElement("div");
      viewer.className = "img-viewer";
      viewer.innerHTML = `<img src="/api/image?ref=${encodeURIComponent(ref)}" alt="">`;
      viewer.onclick = () => viewer.remove();
      document.body.appendChild(viewer);
    }
  });

  $("#btn-config").onclick = () => {
    renderConfig();
    configPanelOpen = true;
    $("#config-panel").hidden = false;
    $("#drawer-mask").hidden = false;
  };

  const closeConfig = () => {
    configPanelOpen = false;
    $("#config-panel").hidden = true;
    $("#drawer-mask").hidden = true;
  };
  $("#btn-config-close").onclick = closeConfig;
  $("#drawer-mask").onclick = closeConfig;

  $("#config-body").addEventListener("change", async (e) => {
    const el = e.target.closest("[data-config]");
    if (!el) return;
    const key = el.dataset.config;
    let value;
    if (el.type === "checkbox") {
      value = el.dataset.invert === "1" ? !el.checked : el.checked;
    } else if (el.type === "number") {
      value = Number(el.value);
      if (!Number.isInteger(value)) {
        toast("请输入整数", true);
        renderConfig();
        return;
      }
    } else {
      value = el.value.trim();
      if (!value) {
        toast("内容不能为空", true);
        renderConfig();
        return;
      }
    }
    try {
      await api("/api/config", { [key]: value });
      state.config[key] = value;
      toast("已保存");
    } catch (err) {
      toast(err.message, true);
      renderConfig();
    }
  });
}

/* ==================== 启动 ==================== */

async function init() {
  bindThemeToggle();
  applyTheme();
  bindEvents();
  await refresh();
  connectEvents();
}

init().catch((err) => toast("加载失败: " + err.message, true));
