/* UI language support (English / 简体中文).
 *
 * The language is chosen from localStorage ('cuterm-lang') if the user has
 * picked one explicitly, otherwise from the browser language. Static markup
 * is translated via data-i18n (textContent) and data-i18n-title (title
 * attribute) attributes; dynamic strings call t(key) directly.
 */
var CUTERM_I18N = {
  'en': {
    'app.new': '+ New',
    'app.newTitle': 'New terminal',
    'app.tagline': 'shared terminal server',
    'app.settings': 'Settings',
    'app.langToggle': '中文',
    'status.running': 'running',
    'status.exited': 'exited',
    'status.clients': '{n} client(s)',
    'term.renameTitle': 'Rename terminal',
    'term.closeTitle': 'Close terminal',
    'term.renamePrompt': 'Rename terminal:',
    'title.exited': ' (exited)',

    'cfg.title': 'cuterm Settings',
    'cfg.subtitle': 'Saved changes take effect immediately and are written to ~/.cuterm/config.json',
    'cfg.language': 'Language',
    'cfg.port': 'Port',
    'cfg.hub': 'cuterm-hub address',
    'cfg.hubNote': 'Connect this node to a cuterm-hub; the hub picks it up automatically, even behind NAT',
    'cfg.hubConnected': 'Connected to the hub',
    'cfg.hubConnecting': 'Connecting to the hub…',
    'cfg.autostart': 'Launch at login',
    'cfg.autostartNote': 'Start cuterm automatically when you log in',
    'cfg.shell': 'Shell',
    'cfg.shellNote': 'Only applies to new terminals',
    'cfg.font': 'Font',
    'cfg.fontSize': 'Font size',
    'cfg.theme': 'Color scheme',
    'cfg.scrollback': 'Scrollback lines',
    'cfg.scrollbackNote': 'Maximum lines kept for scrolling back',
    'cfg.preview': 'Preview',
    'cfg.save': 'Save',
    'cfg.openApp': 'Open app page →',
    'cfg.fetchShellsFail': 'Failed to load the shell list',
    'cfg.fetchAppearanceFail': 'Failed to load the appearance settings',
    'cfg.portRange': 'Port must be between 1 and 65535',
    'cfg.fontSizeRange': 'Font size must be between 6 and 72',
    'cfg.scrollbackRange': 'Scrollback lines must be between 1 and 100000',
    'cfg.unchanged': 'Nothing changed',
    'cfg.saving': 'Saving…',
    'cfg.saved': 'Saved',
    'cfg.savedRedirect': 'Saved, redirecting to the new port…',
    'cfg.saveFailed': 'Save failed: {error}',
    'cfg.previewSample': 'terminal preview'
  },
  'zh-CN': {
    'app.new': '+ 新建',
    'app.newTitle': '新建终端',
    'app.tagline': '共享终端服务器',
    'app.settings': '配置',
    'app.langToggle': 'EN',
    'status.running': '运行中',
    'status.exited': '已退出',
    'status.clients': '{n} 个客户端',
    'term.renameTitle': '重命名终端',
    'term.closeTitle': '关闭终端',
    'term.renamePrompt': '重命名终端：',
    'title.exited': '（已退出）',

    'cfg.title': 'cuterm 配置',
    'cfg.subtitle': '保存后即时生效，并写入 ~/.cuterm/config.json',
    'cfg.language': '界面语言',
    'cfg.port': '端口号',
    'cfg.hub': 'cuterm-hub 地址',
    'cfg.hubNote': '主动连接到 cuterm-hub，hub 会自动添加本节点（可穿透 NAT）',
    'cfg.hubConnected': '已连接到 hub',
    'cfg.hubConnecting': '正在连接 hub…',
    'cfg.autostart': '登录时自动启动',
    'cfg.autostartNote': '登录系统后自动在后台运行 cuterm',
    'cfg.shell': '终端 Shell',
    'cfg.shellNote': '仅对新建终端生效',
    'cfg.font': '字体',
    'cfg.fontSize': '字号',
    'cfg.theme': '配色方案',
    'cfg.scrollback': '滚动缓冲行数',
    'cfg.scrollbackNote': '终端中最多保留的可回滚行数',
    'cfg.preview': '预览',
    'cfg.save': '保存',
    'cfg.openApp': '打开应用页面 →',
    'cfg.fetchShellsFail': '无法获取 shell 列表',
    'cfg.fetchAppearanceFail': '无法获取外观配置',
    'cfg.portRange': '端口号必须在 1 到 65535 之间',
    'cfg.fontSizeRange': '字号必须在 6 到 72 之间',
    'cfg.scrollbackRange': '滚动缓冲行数必须在 1 到 100000 之间',
    'cfg.unchanged': '配置未变化',
    'cfg.saving': '保存中…',
    'cfg.saved': '已保存',
    'cfg.savedRedirect': '已保存，正在跳转到新端口…',
    'cfg.saveFailed': '保存失败：{error}',
    'cfg.previewSample': '终端预览效果'
  }
};

var CUTERM_LANGS = ['en', 'zh-CN'];

/* Current UI language: explicit user choice, else browser language. */
function cutermLang() {
  var saved = null;
  try { saved = localStorage.getItem('cuterm-lang'); } catch (e) { /* private mode */ }
  if (saved && CUTERM_I18N[saved]) return saved;
  var nav = (navigator.language || 'en').toLowerCase();
  return nav.indexOf('zh') === 0 ? 'zh-CN' : 'en';
}

/* Persist an explicit choice and reload so every string re-renders. The
 * choice is also stored on the server, which switches the tray menu and
 * lets other browsers pick it up. */
function cutermSetLang(lang) {
  if (!CUTERM_I18N[lang]) return;
  try { localStorage.setItem('cuterm-lang', lang); } catch (e) { /* ignore */ }
  fetch('/api/language', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ language: lang })
  }).catch(function () { /* tray stays on the old language until next save */ });
  location.reload();
}

function t(key, vars) {
  var lang = cutermLang();
  var s = CUTERM_I18N[lang][key];
  if (s === undefined) s = CUTERM_I18N.en[key];
  if (s === undefined) return key;
  if (vars) {
    for (var k in vars) {
      if (Object.prototype.hasOwnProperty.call(vars, k)) {
        s = s.split('{' + k + '}').join(String(vars[k]));
      }
    }
  }
  return s;
}

/* Pick a language-specific label from a {en, 'zh-CN'} object (themes.js). */
function cutermLabel(label) {
  if (label && typeof label === 'object') return label[cutermLang()] || label.en;
  return label;
}

/* Translate all data-i18n markup. Runs on load; scripts sit at the end of
 * <body> so the DOM is already complete. */
function cutermApplyI18n() {
  document.documentElement.lang = cutermLang();
  var els = document.querySelectorAll('[data-i18n]');
  for (var i = 0; i < els.length; i++) {
    els[i].textContent = t(els[i].getAttribute('data-i18n'));
  }
  var titled = document.querySelectorAll('[data-i18n-title]');
  for (var j = 0; j < titled.length; j++) {
    titled[j].title = t(titled[j].getAttribute('data-i18n-title'));
  }
}
var cutermRenderedLang = cutermLang();
cutermApplyI18n();

/* The server-side language choice (set on the settings page) is shared with
 * the tray menu and other browsers; adopt it when it differs from what this
 * page just rendered with. */
fetch('/api/language').then(function (r) { return r.json(); }).then(function (cfg) {
  if (!cfg.language || !CUTERM_I18N[cfg.language]) return;
  try { localStorage.setItem('cuterm-lang', cfg.language); } catch (e) { /* ignore */ }
  if (cfg.language !== cutermRenderedLang) location.reload();
}).catch(function () { /* offline first render is fine */ });
