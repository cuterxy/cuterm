/* cuterm config page: change the listen port, the shell used by new
 * terminals, and the terminal appearance (font, font size, color scheme).
 * Changes apply immediately and are persisted by the server. */
(function () {
  'use strict';

  var form = document.getElementById('config-form');
  var portInput = document.getElementById('port-input');
  var shellSelect = document.getElementById('shell-select');
  var fontSelect = document.getElementById('font-select');
  var fontSizeInput = document.getElementById('font-size-input');
  var themeSelect = document.getElementById('theme-select');
  var scrollbackInput = document.getElementById('scrollback-input');
  var msg = document.getElementById('config-msg');

  var langSelect = document.getElementById('lang-select');
  langSelect.value = cutermLang();
  langSelect.addEventListener('change', function () {
    cutermSetLang(langSelect.value);
  });

  fetch('/api/version').then(function (r) { return r.json(); }).then(function (d) {
    document.getElementById('cfg-version').textContent = d.version;
  }).catch(function () { /* the empty version line hides itself */ });

  /* ---------- launch at login ---------- */

  var autostartField = document.getElementById('autostart-field');
  var autostartInput = document.getElementById('autostart-input');
  var currentAutostart = null; // null while unsupported or not yet loaded

  fetch('/api/autostart').then(function (r) { return r.json(); }).then(function (a) {
    if (!a.supported) return;
    currentAutostart = !!a.enabled;
    autostartInput.checked = currentAutostart;
    autostartField.hidden = false;
  }).catch(function () { /* leave the field hidden */ });

  var currentShell = null;
  var currentAppearance = null; // {fontFamily, fontSize, theme, scrollback}
  var currentHubAddr = null; // null until GET /api/hub answers

  /* ---------- cuterm-hub connection ---------- */

  var hubAddrInput = document.getElementById('hub-addr-input');
  var hubStatus = document.getElementById('hub-status');

  function renderHubStatus(addr, connected) {
    if (!addr) {
      hubStatus.hidden = true;
      return;
    }
    hubStatus.hidden = false;
    hubStatus.textContent = connected ? t('cfg.hubConnected') : t('cfg.hubConnecting');
  }

  fetch('/api/hub').then(function (r) { return r.json(); }).then(function (h) {
    currentHubAddr = h.addr || '';
    hubAddrInput.value = currentHubAddr;
    renderHubStatus(currentHubAddr, !!h.connected);
  }).catch(function () {
    currentHubAddr = '';
  });

  /* ---------- live preview ---------- */

  var previewTerm = new Terminal({
    cursorBlink: false,
    disableStdin: true,
    scrollback: 0,
    fontSize: CUTERM_DEFAULT_FONT_SIZE,
    fontFamily: CUTERM_DEFAULT_FONT,
    theme: cutermTheme(CUTERM_DEFAULT_THEME).theme
  });
  var previewFit = new FitAddon.FitAddon();
  previewTerm.loadAddon(previewFit);
  var previewBox = document.getElementById('term-preview');
  previewTerm.open(previewBox);
  previewFit.fit();

  // Sample text: a prompt line, styled text, and the 16 ANSI colors.
  previewTerm.write([
    '',
    '\x1b[1;32m$\x1b[0m echo "\x1b[33mHello, cuterm!\x1b[0m"',
    'Hello, \x1b[1;4mcuterm\x1b[0m — ' + t('cfg.previewSample'),
    '',
    '\x1b[31m■ red  \x1b[32m■ green  \x1b[33m■ yellow  \x1b[34m■ blue\x1b[0m',
    '\x1b[35m■ magenta  \x1b[36m■ cyan  \x1b[37m■ white\x1b[0m',
    '',
    '\x1b[40m  \x1b[41m  \x1b[42m  \x1b[43m  \x1b[44m  \x1b[45m  \x1b[46m  \x1b[47m  \x1b[0m normal',
    '\x1b[100m  \x1b[101m  \x1b[102m  \x1b[103m  \x1b[104m  \x1b[105m  \x1b[106m  \x1b[107m  \x1b[0m bright',
    ''
  ].join('\r\n'));

  // Apply the current control values to the preview terminal.
  function updatePreview() {
    var fontSize = parseInt(fontSizeInput.value, 10);
    if (fontSize >= 6 && fontSize <= 72) previewTerm.options.fontSize = fontSize;
    if (fontSelect.value) previewTerm.options.fontFamily = fontSelect.value;
    previewTerm.options.theme = cutermTheme(themeSelect.value || CUTERM_DEFAULT_THEME).theme;
    previewBox.style.background = previewTerm.options.theme.background || '';
    previewFit.fit();
  }

  fontSelect.addEventListener('change', updatePreview);
  fontSizeInput.addEventListener('input', updatePreview);
  themeSelect.addEventListener('change', updatePreview);
  window.addEventListener('resize', function () { previewFit.fit(); });

  function say(text, cls) {
    msg.textContent = text;
    msg.className = cls || '';
  }

  function post(url, body) {
    return fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then(function (r) {
      if (!r.ok) {
        return r.json().then(function (data) {
          throw new Error(data.error || ('failed: ' + r.status));
        });
      }
    });
  }

  /* ---------- load current settings ---------- */

  portInput.value = location.port || (location.protocol === 'https:' ? 443 : 80);

  fetch('/api/shells').then(function (r) { return r.json(); }).then(function (cfg) {
    currentShell = cfg.current;
    cfg.available.forEach(function (sh) {
      var opt = document.createElement('option');
      opt.value = sh;
      opt.textContent = sh;
      if (sh === cfg.current) opt.selected = true;
      shellSelect.appendChild(opt);
    });
  }).catch(function () {
    say(t('cfg.fetchShellsFail'), 'error');
  });

  CUTERM_FONTS.forEach(function (f) {
    var opt = document.createElement('option');
    opt.value = f.value;
    opt.textContent = cutermLabel(f.label);
    fontSelect.appendChild(opt);
  });

  Object.keys(CUTERM_THEMES).forEach(function (key) {
    var opt = document.createElement('option');
    opt.value = key;
    opt.textContent = cutermLabel(CUTERM_THEMES[key].label);
    themeSelect.appendChild(opt);
  });

  fetch('/api/appearance').then(function (r) { return r.json(); }).then(function (a) {
    currentAppearance = {
      fontFamily: a.fontFamily || CUTERM_DEFAULT_FONT,
      fontSize: a.fontSize || CUTERM_DEFAULT_FONT_SIZE,
      theme: a.theme || CUTERM_DEFAULT_THEME,
      scrollback: a.scrollback || CUTERM_DEFAULT_SCROLLBACK
    };
    // A font stack not in the preset list (e.g. edited by hand) still shows.
    if (!Array.prototype.some.call(fontSelect.options, function (o) {
      return o.value === currentAppearance.fontFamily;
    })) {
      var opt = document.createElement('option');
      opt.value = currentAppearance.fontFamily;
      opt.textContent = currentAppearance.fontFamily;
      fontSelect.appendChild(opt);
    }
    fontSelect.value = currentAppearance.fontFamily;
    fontSizeInput.value = currentAppearance.fontSize;
    themeSelect.value = currentAppearance.theme;
    scrollbackInput.value = currentAppearance.scrollback;
    updatePreview();
  }).catch(function () {
    say(t('cfg.fetchAppearanceFail'), 'error');
  });

  /* ---------- save ---------- */

  form.addEventListener('submit', function (ev) {
    ev.preventDefault();
    var port = parseInt(portInput.value, 10);
    if (!port || port < 1 || port > 65535) {
      say(t('cfg.portRange'), 'error');
      return;
    }
    var fontSize = parseInt(fontSizeInput.value, 10);
    if (!fontSize || fontSize < 6 || fontSize > 72) {
      say(t('cfg.fontSizeRange'), 'error');
      return;
    }
    var scrollback = parseInt(scrollbackInput.value, 10);
    if (!scrollback || scrollback < 1 || scrollback > 100000) {
      say(t('cfg.scrollbackRange'), 'error');
      return;
    }

    var portChanged = String(port) !== (location.port || '');
    var shell = shellSelect.value;
    var shellChanged = shell !== currentShell;
    var hubAddr = hubAddrInput.value.trim();
    var hubChanged = currentHubAddr !== null && hubAddr !== currentHubAddr;
    var autostart = autostartInput.checked;
    var autostartChanged = currentAutostart !== null && autostart !== currentAutostart;
    var appearance = {
      fontFamily: fontSelect.value,
      fontSize: fontSize,
      theme: themeSelect.value,
      scrollback: scrollback
    };
    var appearanceChanged = !currentAppearance ||
      appearance.fontFamily !== currentAppearance.fontFamily ||
      appearance.fontSize !== currentAppearance.fontSize ||
      appearance.theme !== currentAppearance.theme ||
      appearance.scrollback !== currentAppearance.scrollback;

    if (!portChanged && !shellChanged && !appearanceChanged && !autostartChanged && !hubChanged) {
      say(t('cfg.unchanged'));
      return;
    }

    say(t('cfg.saving'));
    var chain = Promise.resolve();
    if (appearanceChanged) {
      chain = chain.then(function () { return post('/api/appearance', appearance); });
    }
    if (shellChanged) {
      chain = chain.then(function () { return post('/api/shell', { shell: shell }); });
    }
    if (hubChanged) {
      chain = chain.then(function () { return post('/api/hub', { addr: hubAddr }); });
    }
    if (autostartChanged) {
      chain = chain.then(function () { return post('/api/autostart', { enabled: autostart }); });
    }
    chain.then(function () {
      currentShell = shell;
      currentAppearance = appearance;
      if (hubChanged) {
        currentHubAddr = hubAddr;
        renderHubStatus(hubAddr, false); // reconnect happens in the background
      }
      if (currentAutostart !== null) currentAutostart = autostart;
      if (!portChanged) {
        say(t('cfg.saved'), 'ok');
        return;
      }
      // The server re-listens on the new port; follow it.
      return post('/api/port', { port: port }).then(function () {
        say(t('cfg.savedRedirect'), 'ok');
        location.port = String(port);
      });
    }).catch(function (err) {
      say(t('cfg.saveFailed', { error: err.message }), 'error');
    });
  });
})();
