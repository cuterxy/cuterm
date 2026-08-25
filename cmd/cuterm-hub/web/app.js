/* cuterm-hub web client.
 *
 * The hub aggregates any number of cuterm nodes. REST calls go to
 * /api/nodes/{nodeId}/... and the hub proxies them to the node; attaching to
 * a terminal opens /ws/nodes/{nodeId}/terminals/{id}, which the hub bridges
 * to the node's own terminal WebSocket. The wire protocol is cuterm's,
 * unchanged: binary frames, first byte is the frame type.
 *   0 - output: server -> client, raw bytes written to xterm
 *   1 - input:  client -> server, raw keystroke bytes written to the PTY
 *   2 - resize: client -> server, cols/rows as uint16 big endian
 *   3 - closed: server -> client, terminal process exited
 */
(function () {
  'use strict';

  var FRAME_OUTPUT = 0;
  var FRAME_INPUT = 1;
  var FRAME_RESIZE = 2;
  var FRAME_CLOSED = 3;

  var nodeList = document.getElementById('node-list');
  var termWrap = document.getElementById('term-wrap');
  var termContainer = document.getElementById('terminal');

  // Currently attached session, or null when no terminal is selected.
  var session = null; // {nodeId, id, term, fitAddon, ws, closedByUs, onDetach}
  var nodes = []; // last fetched node statuses
  var termsByNode = {}; // nodeId -> last fetched terminal list
  var collapsedNodes = {}; // nodeId -> true when the terminal list is collapsed
  var draggedNodeId = null; // node id being dragged, while a reorder drag runs

  // Terminal display settings, kept in sync with the hub config so that
  // changes made on the config page apply here without a reload.
  var appearance = {
    fontFamily: CUTERM_DEFAULT_FONT,
    fontSize: CUTERM_DEFAULT_FONT_SIZE,
    theme: CUTERM_DEFAULT_THEME,
    scrollback: CUTERM_DEFAULT_SCROLLBACK
  };

  function termTheme() {
    return cutermTheme(appearance.theme).theme;
  }

  function applyTermBackground() {
    termContainer.style.background = termTheme().background || '';
  }

  function fetchAppearance() {
    return fetch('/api/appearance').then(function (r) { return r.json(); }).then(function (a) {
      var next = {
        fontFamily: a.fontFamily || CUTERM_DEFAULT_FONT,
        fontSize: a.fontSize || CUTERM_DEFAULT_FONT_SIZE,
        theme: a.theme || CUTERM_DEFAULT_THEME,
        scrollback: a.scrollback || CUTERM_DEFAULT_SCROLLBACK
      };
      if (next.fontFamily === appearance.fontFamily &&
          next.fontSize === appearance.fontSize &&
          next.theme === appearance.theme &&
          next.scrollback === appearance.scrollback) {
        return;
      }
      appearance = next;
      applyTermBackground();
      if (session) {
        session.term.options.fontFamily = appearance.fontFamily;
        session.term.options.fontSize = appearance.fontSize;
        session.term.options.theme = termTheme();
        session.term.options.scrollback = appearance.scrollback;
        // Font changes alter the cell size; refit and tell the PTY.
        session.fitAddon.fit();
        sendResize();
      }
    }).catch(function () { /* server briefly unreachable; keep current */ });
  }

  /* ---------- REST ---------- */

  function fetchNodes() {
    return fetch('/api/nodes').then(function (r) { return r.json(); });
  }

  function fetchTerminals(nodeId) {
    return fetch('/api/nodes/' + nodeId + '/terminals').then(function (r) { return r.json(); });
  }

  function createTerminal(nodeId, name) {
    return fetch('/api/nodes/' + nodeId + '/terminals', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name || '' })
    }).then(function (r) {
      if (!r.ok) throw new Error('create failed: ' + r.status);
      return r.json();
    });
  }

  function closeTerminal(nodeId, id) {
    return fetch('/api/nodes/' + nodeId + '/terminals/' + id, { method: 'DELETE' });
  }

  function renameTerminal(nodeId, id, name) {
    return fetch('/api/nodes/' + nodeId + '/terminals/' + id, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name })
    }).then(function (r) {
      if (!r.ok) throw new Error('rename failed: ' + r.status);
    });
  }

  function addNode(name, addr) {
    return fetch('/api/nodes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name, addr: addr })
    }).then(function (r) {
      if (!r.ok) {
        return r.json().then(function (data) {
          throw new Error(data.error || ('add failed: ' + r.status));
        });
      }
    });
  }

  function removeNode(id) {
    return fetch('/api/nodes/' + id, { method: 'DELETE' }).then(function (r) {
      if (!r.ok) {
        return r.json().then(function (data) {
          throw new Error(data.error || ('remove failed: ' + r.status));
        });
      }
    });
  }

  /* ---------- list rendering ---------- */

  function renderTermItem(node, item) {
    var li = document.createElement('li');
    li.className = 'term-item' +
      (session && session.nodeId === node.id && session.id === item.id ? ' active' : '');

    var dot = document.createElement('span');
    dot.className = 'dot ' + (item.alive ? 'alive' : 'dead');

    var meta = document.createElement('div');
    meta.className = 'meta';
    var name = document.createElement('div');
    name.className = 'name';
    name.textContent = item.name;
    var sub = document.createElement('div');
    sub.className = 'sub';
    sub.textContent = (item.alive ? t('status.running') : t('status.exited')) +
      ' · ' + item.shell +
      ' · ' + t('status.clients', { n: item.clients });
    meta.appendChild(name);
    meta.appendChild(sub);

    var renameBtn = document.createElement('button');
    renameBtn.className = 'icon-btn';
    renameBtn.title = t('term.renameTitle');
    renameBtn.textContent = '✎';
    renameBtn.addEventListener('click', function (ev) {
      ev.stopPropagation();
      var newName = window.prompt(t('term.renamePrompt'), item.name);
      if (newName === null) return;
      newName = newName.trim();
      if (!newName || newName === item.name) return;
      renameTerminal(node.id, item.id, newName).then(function () {
        refresh();
      }).catch(function (err) { window.alert(err.message); });
    });

    var closeBtn = document.createElement('button');
    closeBtn.className = 'icon-btn close-btn';
    closeBtn.title = t('term.closeTitle');
    closeBtn.textContent = '×';
    closeBtn.addEventListener('click', function (ev) {
      ev.stopPropagation();
      closeTerminal(node.id, item.id).then(function () {
        if (session && session.nodeId === node.id && session.id === item.id) detach();
        refresh();
      });
    });

    li.appendChild(dot);
    li.appendChild(meta);
    li.appendChild(renameBtn);
    li.appendChild(closeBtn);
    li.addEventListener('click', function () { attach(node.id, item.id); });
    return li;
  }

  // Move the dragged node before/after the target among the visible nodes,
  // then persist the new order. Hidden nodes keep their registry slots
  // (the server does the same), so they simply ride along.
  function reorderNode(dragId, targetId, after) {
    if (dragId === targetId) return;
    var byId = {};
    nodes.forEach(function (n) { byId[n.id] = n; });
    var order = [];
    nodes.forEach(function (n) {
      if (!n.hidden && n.id !== dragId) order.push(n.id);
    });
    var at = order.indexOf(targetId);
    if (at < 0) return;
    order.splice(after ? at + 1 : at, 0, dragId);

    var queue = order.slice();
    nodes = nodes.map(function (n) {
      return n.hidden ? n : byId[queue.shift()];
    });
    renderList();

    fetch('/api/nodes/order', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids: order })
    }).then(function (r) {
      if (!r.ok) refresh(); // server rejected the order; resync
    }).catch(function () { refresh(); });
  }

  function clearDropMark(li) {
    li.classList.remove('drop-before');
    li.classList.remove('drop-after');
  }

  function renderNode(node) {
    var li = document.createElement('li');
    li.className = 'node';

    // Drag to reorder the node list.
    li.draggable = true;
    li.addEventListener('dragstart', function (ev) {
      draggedNodeId = node.id;
      ev.dataTransfer.effectAllowed = 'move';
      ev.dataTransfer.setData('text/plain', node.id);
    });
    li.addEventListener('dragend', function () {
      draggedNodeId = null;
      // A cancelled drag may leave the mark on some other node's item.
      Array.prototype.forEach.call(nodeList.querySelectorAll('.node'), clearDropMark);
    });
    li.addEventListener('dragover', function (ev) {
      if (!draggedNodeId || draggedNodeId === node.id) return;
      ev.preventDefault();
      ev.dataTransfer.dropEffect = 'move';
      var rect = li.getBoundingClientRect();
      var before = ev.clientY < rect.top + rect.height / 2;
      li.classList.toggle('drop-before', before);
      li.classList.toggle('drop-after', !before);
    });
    li.addEventListener('dragleave', function () { clearDropMark(li); });
    li.addEventListener('drop', function (ev) {
      ev.preventDefault();
      var rect = li.getBoundingClientRect();
      var after = ev.clientY >= rect.top + rect.height / 2;
      clearDropMark(li);
      if (draggedNodeId) reorderNode(draggedNodeId, node.id, after);
    });

    var head = document.createElement('div');
    head.className = 'node-head';

    var caret = document.createElement('span');
    caret.className = 'caret';
    caret.textContent = collapsedNodes[node.id] ? '▲' : '▼';

    var meta = document.createElement('div');
    meta.className = 'meta';
    var name = document.createElement('div');
    name.className = 'name';
    name.textContent = node.name;
    var sub = document.createElement('div');
    sub.className = 'sub';
    sub.textContent = (node.reverse ? t('node.reverse') : node.addr) + (node.online ? '' : ' · ' + t('node.offline'));
    meta.appendChild(name);
    meta.appendChild(sub);

    var newBtn = document.createElement('button');
    newBtn.className = 'icon-btn';
    newBtn.title = t('node.newTitle');
    newBtn.textContent = '+';
    newBtn.disabled = !node.online;
    newBtn.addEventListener('click', function (ev) {
      ev.stopPropagation();
      createTerminal(node.id, '').then(function (info) {
        refresh().then(function () { attach(node.id, info.id); });
      }).catch(function (err) { window.alert(err.message); });
    });

    head.appendChild(caret);
    head.appendChild(meta);
    head.appendChild(newBtn);
    head.addEventListener('click', function () {
      collapsedNodes[node.id] = !collapsedNodes[node.id];
      renderList();
    });
    // Removing a connected reverse node hides it instead of deleting it: the
    // hub keeps it registered and the config page can re-enable it.
    var removeBtn = document.createElement('button');
    removeBtn.className = 'icon-btn close-btn';
    removeBtn.title = t('cfg.removeNodeTitle');
    removeBtn.textContent = '×';
    removeBtn.addEventListener('click', function (ev) {
      ev.stopPropagation();
      removeNode(node.id).then(function () {
        if (session && session.nodeId === node.id) detach();
        delete termsByNode[node.id];
        refresh();
      }).catch(function (err) {
        window.alert(t('cfg.nodeRemoveFail', { error: err.message }));
      });
    });
    head.appendChild(removeBtn);
    li.appendChild(head);

    var terms = termsByNode[node.id] || [];
    if (!collapsedNodes[node.id] && node.online && terms.length > 0) {
      var ul = document.createElement('ul');
      ul.className = 'node-terms';
      terms.forEach(function (item) {
        ul.appendChild(renderTermItem(node, item));
      });
      li.appendChild(ul);
    }
    return li;
  }

  function renderList() {
    nodeList.innerHTML = '';
    nodes.forEach(function (node) {
      if (node.hidden) return;
      nodeList.appendChild(renderNode(node));
    });
  }

  function refresh() {
    return fetchNodes().then(function (list) {
      nodes = list;
      var jobs = [];
      list.forEach(function (node) {
        if (!node.online) {
          termsByNode[node.id] = [];
          return;
        }
        jobs.push(fetchTerminals(node.id).then(function (terms) {
          termsByNode[node.id] = terms;
        }).catch(function () { /* keep the previous list for this node */ }));
      });
      return Promise.all(jobs).then(function () {
        renderList();
        // The terminal we are attached to may have exited elsewhere, or its
        // node may have gone offline.
        if (session) {
          var mine = null;
          var terms = termsByNode[session.nodeId] || [];
          for (var i = 0; i < terms.length; i++) {
            if (terms[i].id === session.id) { mine = terms[i]; break; }
          }
          if (!mine) {
            detach();
          } else {
            document.title = mine.name + (mine.alive ? '' : t('title.exited')) + ' - cuterm-hub';
          }
        }
      });
    }).catch(function () { /* server briefly unreachable; try next tick */ });
  }

  /* ---------- attach / detach ---------- */

  function attach(nodeId, id) {
    if (session && session.nodeId === nodeId && session.id === id) return;
    detach();

    // Unhide before open/fit: fit() measures the container, which has no
    // dimensions while #term-wrap is display:none.
    termWrap.hidden = false;

    var term = new Terminal({
      cursorBlink: true,
      fontSize: appearance.fontSize,
      fontFamily: appearance.fontFamily,
      theme: termTheme(),
      scrollback: appearance.scrollback
    });
    var fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(termContainer);
    fitAddon.fit();
    applyTermBackground();

    // Keep the viewport pinned to the bottom while output streams in. TUIs
    // that redraw with ANSI sequences (e.g. kimi) can yank xterm's viewport
    // to the top of the buffer; unless the user deliberately scrolled away
    // from the bottom, snap back to the latest output after each write.
    var pinnedToBottom = true;
    var mouseDown = false;
    var viewport = termContainer.querySelector('.xterm-viewport');
    viewport.addEventListener('mousedown', function () { mouseDown = true; });
    var onMouseUp = function () { mouseDown = false; };
    window.addEventListener('mouseup', onMouseUp);
    viewport.addEventListener('wheel', function (ev) {
      // Wheeling up means the user wants to read scrollback; stop pinning.
      // Reaching the bottom again (scroll event below) re-enables pinning.
      if (ev.deltaY < 0) pinnedToBottom = false;
    }, { passive: true });
    term.attachCustomKeyEventHandler(function (ev) {
      if (ev.type === 'keydown' && ev.shiftKey &&
          (ev.key === 'PageUp' || ev.key === 'Home')) {
        pinnedToBottom = false;
      }
      return true;
    });

    // Auto-hide scrollbar: show it while scrolling, fade out after 1s idle.
    // The viewport element is destroyed by term.dispose(), so no cleanup needed.
    var scrollHideTimer = null;
    viewport.addEventListener('scroll', function () {
      // Landing near the bottom re-pins; dragging the scrollbar up unpins.
      var atBottom = viewport.scrollTop >=
        viewport.scrollHeight - viewport.clientHeight - 1;
      if (atBottom) pinnedToBottom = true;
      else if (mouseDown) pinnedToBottom = false;
      termContainer.classList.add('scrolling');
      clearTimeout(scrollHideTimer);
      scrollHideTimer = setTimeout(function () {
        termContainer.classList.remove('scrolling');
      }, 1000);
    });

    session = {
      nodeId: nodeId, id: id, term: term, fitAddon: fitAddon, ws: null,
      closedByUs: false,
      onDetach: function () { window.removeEventListener('mouseup', onMouseUp); }
    };

    var proto = location.protocol === 'https:' ? 'wss' : 'ws';
    var ws = new WebSocket(proto + '://' + location.host +
      '/ws/nodes/' + nodeId + '/terminals/' + id);
    ws.binaryType = 'arraybuffer';
    session.ws = ws;

    ws.onopen = function () { sendResize(); };
    ws.onmessage = function (ev) {
      var data = new Uint8Array(ev.data);
      if (data.length === 0) return;
      var type = data[0];
      var payload = data.subarray(1);
      if (type === FRAME_OUTPUT) {
        term.write(payload, function () {
          if (session && session.term === term && pinnedToBottom) {
            term.scrollToBottom();
          }
        });
      } else if (type === FRAME_CLOSED) {
        term.write('\r\n\x1b[90m--- ' + new TextDecoder().decode(payload) + ' ---\x1b[0m\r\n', function () {
          if (session && session.term === term && pinnedToBottom) {
            term.scrollToBottom();
          }
        });
        refresh();
      }
    };
    ws.onclose = function () {
      if (session && session.ws === ws && !session.closedByUs) refresh();
    };

    term.onData(function (d) {
      if (ws.readyState === WebSocket.OPEN) {
        sendFrame(ws, FRAME_INPUT, new TextEncoder().encode(d));
      }
    });
    term.onResize(function () { sendResize(); });

    // The first fit() above can run before layout settles; refit on the next
    // frame so rows/cols always match the real container size.
    requestAnimationFrame(function () {
      if (session && session.term === term) {
        fitAddon.fit();
        sendResize();
      }
    });

    var info = null;
    var terms = termsByNode[nodeId] || [];
    for (var i = 0; i < terms.length; i++) {
      if (terms[i].id === id) { info = terms[i]; break; }
    }
    document.title = (info ? info.name : id) + ' - cuterm-hub';
    renderList();
    term.focus();
  }

  function detach() {
    if (!session) return;
    session.closedByUs = true;
    if (session.ws && session.ws.readyState !== WebSocket.CLOSED) session.ws.close();
    if (session.onDetach) session.onDetach();
    session.term.dispose();
    session = null;
    document.title = 'cuterm-hub';
    termWrap.hidden = true;
    renderList();
  }

  function sendFrame(ws, type, payload) {
    var out = new Uint8Array(1 + payload.length);
    out[0] = type;
    out.set(payload, 1);
    ws.send(out);
  }

  function sendResize() {
    if (!session) return;
    var ws = session.ws;
    if (ws.readyState !== WebSocket.OPEN) return;
    var payload = new Uint8Array(4);
    payload[0] = session.term.cols >> 8;
    payload[1] = session.term.cols & 0xff;
    payload[2] = session.term.rows >> 8;
    payload[3] = session.term.rows & 0xff;
    sendFrame(ws, FRAME_RESIZE, payload);
  }

  /* ---------- events ---------- */

  document.getElementById('lang-toggle').addEventListener('click', function (ev) {
    ev.preventDefault();
    cutermSetLang(cutermLang() === 'zh-CN' ? 'en' : 'zh-CN');
  });

  var addToggle = document.getElementById('node-add-toggle');
  var addForm = document.getElementById('node-add-form');
  var addName = document.getElementById('app-node-name');
  var addAddr = document.getElementById('app-node-addr');

  addToggle.addEventListener('click', function () {
    addForm.hidden = !addForm.hidden;
    if (!addForm.hidden) addAddr.focus();
  });

  addForm.addEventListener('submit', function (ev) {
    ev.preventDefault();
    var addr = addAddr.value.trim();
    if (!addr) {
      window.alert(t('cfg.nodeAddrRequired'));
      return;
    }
    addNode(addName.value.trim(), addr).then(function () {
      addName.value = '';
      addAddr.value = '';
      addForm.hidden = true;
      refresh();
    }).catch(function (err) {
      window.alert(t('cfg.nodeAddFail', { error: err.message }));
    });
  });

  window.addEventListener('resize', function () {
    if (session) {
      session.fitAddon.fit();
      sendResize();
    }
  });

  /* ---------- boot ---------- */

  fetch('/api/version').then(function (r) { return r.json(); }).then(function (d) {
    document.getElementById('app-version').textContent = d.version;
  }).catch(function () { /* leave the footer without a version */ });

  refresh();
  fetchAppearance();
  window.setInterval(function () {
    refresh();
    fetchAppearance();
  }, 3000);
})();
