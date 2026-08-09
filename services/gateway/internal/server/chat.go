package server

import (
	"net/http"

	"github.com/ai-dos/foundation/errors"
)

// chatHTML is the static browser chat UI served at "/chat". It is a
// pure client for POST /v1/chat/completions on the same origin: the
// page itself is public, holds no secrets, and every request it makes
// still passes the normal auth and rate-limit middleware. The caller's
// API key lives only in their browser's localStorage.
//
// The UI is a five-state machine — idle / working / waiting / error /
// done — driven by the real request lifecycle (there is no agent
// runtime behind it yet, so the states map to what actually happens:
// an in-flight completion with abort, a key that needs the user, a
// failed call with retry). When an agent runtime lands in later
// phases, it plugs into these same states without a UI rewrite.
//
// No template engine and no JS backticks: this constant is a Go raw
// string, so a literal backtick would terminate it — the code-fence
// marker is assembled from char codes instead.
const chatHTML = `<!doctype html>
<html lang="ar" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🧠</text></svg>">
<title>Mini AI-DOS — شات</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; margin: 0; display: flex; flex-direction: column; height: 100dvh; }
  header { padding: .7rem 1rem; border-bottom: 1px solid rgba(127,127,127,.25); display: flex; align-items: center; gap: .6rem; flex-wrap: wrap; }
  header h1 { font-size: 1.05rem; margin: 0; }
  #statuschip { font-size: .72rem; padding: .18rem .6rem; border-radius: 999px; border: 1px solid rgba(127,127,127,.35); opacity: .85; }
  #statuschip.working { color: #3b82f6; border-color: #3b82f6; }
  #statuschip.waiting { color: #d97706; border-color: #d97706; }
  #statuschip.error { color: #d33; border-color: #d33; }
  #statuschip.done { color: #16a34a; border-color: #16a34a; }
  #prov { font-size: .72rem; opacity: .6; direction: ltr; margin-inline-start: auto; }
  header button { font-size: .78rem; padding: .3rem .7rem; border-radius: 6px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; cursor: pointer; }
  #log { flex: 1; overflow-y: auto; padding: 1rem; display: flex; flex-direction: column; gap: .6rem; }
  .msg { max-width: 46rem; padding: .6rem .9rem; border-radius: 12px; line-height: 1.6; }
  .msg .txt { white-space: pre-wrap; }
  .user { align-self: flex-start; background: rgba(59,130,246,.18); }
  .assistant { align-self: flex-end; background: rgba(127,127,127,.15); }
  .err { align-self: center; color: #d33; font-size: .85rem; }
  .card { align-self: flex-end; background: rgba(127,127,127,.12); border: 1px solid rgba(127,127,127,.25); border-radius: 12px; padding: .7rem .9rem; min-width: 15rem; }
  .card .ctitle { font-size: .85rem; font-weight: 600; margin-bottom: .45rem; }
  .card.working .ctitle { color: #3b82f6; }
  .card.waiting .ctitle { color: #d97706; }
  .card.error .ctitle { color: #d33; }
  .steps { display: flex; flex-direction: column; gap: .25rem; font-size: .82rem; margin-bottom: .55rem; }
  .step { opacity: .5; }
  .step.active { opacity: 1; color: #3b82f6; }
  .step.done { opacity: .8; color: #16a34a; }
  .cmsg { font-size: .82rem; margin-bottom: .55rem; }
  .activity { font-family: ui-monospace, monospace; font-size: .72rem; direction: ltr; text-align: left; max-height: 8rem; overflow-y: auto; background: rgba(0,0,0,.18); border-radius: 6px; padding: .35rem .5rem; margin-bottom: .5rem; }
  @media (prefers-color-scheme: light) { .activity { background: rgba(0,0,0,.05); } }
  .activity div { white-space: pre-wrap; opacity: .85; }
  .actions { display: flex; gap: .5rem; }
  .actions button { font-size: .78rem; padding: .3rem .9rem; border-radius: 6px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; cursor: pointer; }
  .actions .primary { background: #3b82f6; border-color: #3b82f6; color: #fff; }
  .actions .danger { border-color: #d33; color: #d33; }
  .code { position: relative; direction: ltr; text-align: left; margin: .45rem 0; }
  .code pre { overflow-x: auto; background: rgba(0,0,0,.25); padding: .7rem .8rem; padding-top: 1.6rem; border-radius: 8px; margin: 0; font-size: .85rem; }
  @media (prefers-color-scheme: light) { .code pre { background: rgba(0,0,0,.07); } }
  .code .lang { position: absolute; top: .3rem; left: .6rem; font-size: .68rem; opacity: .55; text-transform: lowercase; }
  .copybtn { position: absolute; top: .25rem; right: .35rem; font-size: .7rem; padding: .15rem .55rem; border-radius: 5px; border: 1px solid rgba(127,127,127,.4); background: rgba(127,127,127,.15); color: inherit; cursor: pointer; }
  .meta { font-size: .7rem; opacity: .6; margin-top: .4rem; direction: ltr; text-align: left; }
  .files { margin-top: .5rem; border-top: 1px dashed rgba(127,127,127,.3); padding-top: .4rem; }
  .files .ftitle { font-size: .74rem; opacity: .7; margin-bottom: .3rem; }
  .filebtn { display: inline-block; direction: ltr; font-size: .74rem; margin: .12rem; padding: .12rem .5rem; border-radius: 5px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; cursor: pointer; font-family: ui-monospace, monospace; }
  .zipbtn { display: block; margin: .3rem 0 .5rem; background: rgba(59,130,246,.15); border-color: #3b82f6; }
  .zipbtn:disabled { opacity: .6; cursor: default; }
  .filerow { display: inline-flex; align-items: stretch; margin: .12rem; direction: ltr; }
  .filerow .filebtn { margin: 0; border-top-right-radius: 0; border-bottom-right-radius: 0; }
  .dlbtn { border: 1px solid rgba(127,127,127,.4); border-left: 0; border-radius: 0; background: rgba(127,127,127,.12); color: inherit; cursor: pointer; font-size: .74rem; padding: 0 .45rem; }
  .filerow .dlbtn:last-child { border-top-right-radius: 5px; border-bottom-right-radius: 5px; }
  .preview { margin-top: .5rem; }
  .preview iframe { width: 100%; height: 22rem; border: 1px solid rgba(127,127,127,.35); border-radius: 8px; background: #fff; }
  form { display: flex; gap: .5rem; padding: .8rem 1rem; border-top: 1px solid rgba(127,127,127,.25); }
  textarea { flex: 1; resize: none; padding: .6rem .8rem; border-radius: 10px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; font: inherit; height: 3rem; }
  form button { padding: 0 1.2rem; border-radius: 10px; border: 0; background: #3b82f6; color: #fff; font: inherit; cursor: pointer; }
  form button:disabled { opacity: .5; }
  #modelabel { display: flex; align-items: center; gap: .3rem; font-size: .78rem; opacity: .85; user-select: none; cursor: pointer; white-space: nowrap; }
</style>
</head>
<body>
<header>
  <h1>Mini AI-DOS</h1>
  <span id="statuschip">جاهز</span>
  <span id="prov"></span>
  <button id="newbtn" type="button">محادثة جديدة</button>
  <button id="keybtn" type="button">تغيير المفتاح</button>
</header>
<div id="log"></div>
<form id="f">
  <textarea id="in" placeholder="اكتب رسالتك… (Enter للإرسال، Shift+Enter لسطر جديد)"></textarea>
  <label id="modelabel"><input type="checkbox" id="agentmode"> 🤖 وكيل</label>
  <button id="send" type="submit">إرسال</button>
</form>
<script>
(function () {
  var KEY = 'aidos_api_key';
  var log = document.getElementById('log');
  var form = document.getElementById('f');
  var input = document.getElementById('in');
  var sendBtn = document.getElementById('send');
  var chip = document.getElementById('statuschip');
  var msgs = [];
  var memKey = '';

  // ---- The status machine. Everything the UI shows derives from
  // state.status; adding future agent states means adding a label and
  // a card builder, not restructuring the page.
  var state = { status: 'idle', lastText: '', ctrl: null, card: null, retryFn: null, runId: null, poll: null };
  var LABELS = { idle: 'جاهز', working: 'شغال…', waiting: 'مستني موافقتك', error: 'خطأ', done: 'اكتمل ✓' };

  function setStatus(s) {
    state.status = s;
    chip.textContent = LABELS[s] || s;
    chip.className = s;
    sendBtn.disabled = (s === 'working');
    if (s === 'done') { setTimeout(function () { if (state.status === 'done') setStatus('idle'); }, 2500); }
  }

  function dropCard() {
    if (state.card) { state.card.remove(); state.card = null; }
  }

  // ---- Key handling (localStorage with in-memory fallback for
  // strict privacy modes that make storage throw).
  function loadKey() {
    try { return localStorage.getItem(KEY) || memKey; } catch (e) { return memKey; }
  }
  function storeKey(k) {
    memKey = k;
    try { localStorage.setItem(KEY, k); } catch (e) {}
  }
  function clearKey() {
    memKey = '';
    try { localStorage.removeItem(KEY); } catch (e) {}
  }
  function promptKey() {
    var k = (window.prompt('حط الـ API key بتاع الـ gateway (بيتخزن في متصفحك بس):') || '').trim();
    if (k) storeKey(k);
    return k;
  }

  fetch('/health').then(function (r) { return r.json(); }).then(function (h) {
    if (h && h.provider) { document.getElementById('prov').textContent = 'provider: ' + h.provider; }
  }).catch(function () {});

  // ---- Rendering helpers (all textContent — model output is never
  // parsed as HTML). The fence marker is built from char codes
  // because this page lives inside a Go raw string.
  var FENCE = String.fromCharCode(96, 96, 96);

  function copyText(txt, btn) {
    function done() {
      var old = btn.textContent;
      btn.textContent = 'اتنسخ ✓';
      setTimeout(function () { btn.textContent = old; }, 1500);
    }
    function fallback() {
      var ta = document.createElement('textarea');
      ta.value = txt;
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); done(); } catch (e) {}
      ta.remove();
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).then(done).catch(fallback);
    } else { fallback(); }
  }

  function renderContent(el, text) {
    var parts = text.split(FENCE);
    for (var i = 0; i < parts.length; i++) {
      if (i % 2 === 0) {
        if (!parts[i]) continue;
        var t = document.createElement('div');
        t.className = 'txt';
        t.dir = 'auto';
        t.textContent = parts[i];
        el.appendChild(t);
      } else {
        var code = parts[i];
        var lang = '';
        var nl = code.indexOf('\n');
        if (nl > -1) {
          var first = code.slice(0, nl).trim();
          if (first.length < 20 && first.indexOf(' ') === -1) { lang = first; code = code.slice(nl + 1); }
        }
        var wrap = document.createElement('div');
        wrap.className = 'code';
        if (lang) {
          var lb = document.createElement('span');
          lb.className = 'lang';
          lb.textContent = lang;
          wrap.appendChild(lb);
        }
        var btn = document.createElement('button');
        btn.className = 'copybtn';
        btn.type = 'button';
        btn.textContent = 'نسخ';
        (function (c, b) { b.onclick = function () { copyText(c, b); }; })(code, btn);
        wrap.appendChild(btn);
        var pre = document.createElement('pre');
        var codeEl = document.createElement('code');
        codeEl.textContent = code;
        pre.appendChild(codeEl);
        wrap.appendChild(pre);
        el.appendChild(wrap);
      }
    }
  }

  function add(role, text) {
    var d = document.createElement('div');
    d.className = 'msg ' + role;
    if (role === 'assistant') { renderContent(d, text); }
    else {
      var t = document.createElement('div');
      t.className = 'txt';
      t.dir = 'auto';
      t.textContent = text;
      d.appendChild(t);
    }
    log.appendChild(d);
    log.scrollTop = log.scrollHeight;
    return d;
  }

  function addMeta(bubble, data, resp, durMs) {
    var bits = [];
    if (data.model) { bits.push(data.model); }
    if (data.usage) { bits.push('tokens ' + data.usage.prompt_tokens + '↑ ' + data.usage.completion_tokens + '↓'); }
    bits.push((durMs / 1000).toFixed(1) + 's');
    var rem = resp.headers.get('X-RateLimit-Remaining');
    var lim = resp.headers.get('X-RateLimit-Limit');
    if (rem !== null && lim !== null) { bits.push('limit ' + rem + '/' + lim + ' left'); }
    var rid = resp.headers.get('X-Request-Id');
    if (rid) { bits.push('req ' + rid.slice(0, 8)); }
    var m = document.createElement('div');
    m.className = 'meta';
    m.textContent = bits.join(' · ');
    bubble.appendChild(m);
  }

  // ---- Cards: one per non-idle status, all built the same way.
  function card(kind, title) {
    dropCard();
    var c = document.createElement('div');
    c.className = 'card ' + kind;
    var h = document.createElement('div');
    h.className = 'ctitle';
    h.textContent = title;
    c.appendChild(h);
    log.appendChild(c);
    log.scrollTop = log.scrollHeight;
    state.card = c;
    return c;
  }

  function actionBtn(label, cls, fn) {
    var b = document.createElement('button');
    b.type = 'button';
    b.textContent = label;
    if (cls) b.className = cls;
    b.onclick = fn;
    return b;
  }

  function workingCard() {
    var c = card('working', '● شغال');
    var steps = document.createElement('div');
    steps.className = 'steps';
    var s1 = document.createElement('div'); s1.className = 'step done'; s1.textContent = '✓ جهزت الطلب';
    var s2 = document.createElement('div'); s2.className = 'step active'; s2.textContent = '● مستني رد الموديل';
    var s3 = document.createElement('div'); s3.className = 'step'; s3.textContent = '○ معالجة الرد';
    steps.appendChild(s1); steps.appendChild(s2); steps.appendChild(s3);
    c.appendChild(steps);
    var a = document.createElement('div');
    a.className = 'actions';
    a.appendChild(actionBtn('إيقاف', 'danger', function () {
      if (state.ctrl) state.ctrl.abort();
    }));
    c.appendChild(a);
    return { gotHeaders: function () {
      s2.className = 'step done'; s2.textContent = '✓ وصل رد الموديل';
      s3.className = 'step active'; s3.textContent = '● معالجة الرد';
    } };
  }

  function waitingCard(msg, onApprove) {
    setStatus('waiting');
    var c = card('waiting', '⏸ محتاج حاجة منك');
    var m = document.createElement('div');
    m.className = 'cmsg';
    m.textContent = msg;
    c.appendChild(m);
    var a = document.createElement('div');
    a.className = 'actions';
    a.appendChild(actionBtn('أدخل المفتاح', 'primary', function () {
      var k = promptKey();
      if (k) { dropCard(); onApprove(); }
    }));
    a.appendChild(actionBtn('إلغاء', '', function () { dropCard(); setStatus('idle'); }));
    c.appendChild(a);
  }

  function errorCard(msg) {
    setStatus('error');
    var c = card('error', '⚠ حصل خطأ');
    var m = document.createElement('div');
    m.className = 'cmsg';
    m.textContent = msg;
    c.appendChild(m);
    var a = document.createElement('div');
    a.className = 'actions';
    a.appendChild(actionBtn('إعادة المحاولة', 'primary', function () {
      dropCard();
      if (state.retryFn) { state.retryFn(); } else { send(state.lastText); }
    }));
    a.appendChild(actionBtn('إلغاء', '', function () { dropCard(); setStatus('idle'); }));
    c.appendChild(a);
  }

  // ---- Flow 1: plain chat send.
  function send(text) {
    if (!text) return;
    var key = loadKey();
    if (!key) {
      state.lastText = text;
      waitingCard('محتاج الـ API key عشان أكمل.', function () { send(text); });
      return;
    }
    state.lastText = text;
    state.retryFn = function () { send(text); };
    msgs.push({ role: 'user', content: text });
    add('user', text);
    setStatus('working');
    var w = workingCard();
    var t0 = Date.now();
    state.ctrl = new AbortController();
    fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + key },
      body: JSON.stringify({ messages: msgs }),
      signal: state.ctrl.signal
    }).then(function (r) {
      w.gotHeaders();
      return r.json().then(function (data) { return { ok: r.ok, status: r.status, resp: r, data: data }; });
    }).then(function (res) {
      dropCard();
      if (!res.ok) {
        msgs.pop();
        if (res.status === 401) {
          clearKey();
          waitingCard('المفتاح مرفوض — أدخل مفتاح صحيح عشان أكمل.', function () { send(state.lastText); });
          return;
        }
        errorCard((res.data && res.data.error && res.data.error.message) || ('HTTP ' + res.status));
        return;
      }
      var reply = res.data.choices[0].message;
      msgs.push(reply);
      var bubble = add('assistant', reply.content);
      addMeta(bubble, res.data, res.resp, Date.now() - t0);
      log.scrollTop = log.scrollHeight;
      setStatus('done');
    }).catch(function (err) {
      dropCard();
      msgs.pop();
      if (err && err.name === 'AbortError') {
        input.value = state.lastText;
        setStatus('idle');
        input.focus();
        return;
      }
      errorCard('فشل الاتصال: ' + err.message);
    }).finally(function () {
      state.ctrl = null;
      if (state.status === 'working') setStatus('idle');
      input.focus();
    });
  }

  // ---- Flow 2: agent runs. POST creates a run; the working card is
  // then driven by polled snapshots — live plan steps, real phases,
  // real cancellation.
  var PHASES = {
    planning: 'بيفهم المهمة وبيخطط…',
    planned: 'الخطة جاهزة — راجعها واضغط ابدأ:',
    executing: 'بينفذ الخطة…',
    inspecting: 'بيراجع شغله…',
    fixing: 'بيصلح المشاكل اللي لقاها…',
    awaiting_approval: '⚠ الوكيل محتاج موافقتك:'
  };

  function renderSteps(stepsEl, steps) {
    stepsEl.textContent = '';
    for (var i = 0; i < steps.length; i++) {
      var st = steps[i].status;
      var d = document.createElement('div');
      d.className = 'step' + (st === 'active' ? ' active' : (st === 'done' ? ' done' : ''));
      d.dir = 'auto';
      d.textContent = (st === 'done' ? '✓ ' : (st === 'active' ? '● ' : '○ ')) + steps[i].title;
      stepsEl.appendChild(d);
    }
  }

  function agentSend(text) {
    if (!text) return;
    var key = loadKey();
    if (!key) {
      state.lastText = text;
      waitingCard('محتاج الـ API key عشان أكمل.', function () { agentSend(text); });
      return;
    }
    state.lastText = text;
    state.retryFn = function () { agentSend(text); };
    add('user', text);
    setStatus('working');
    var c = card('working', '🤖 الوكيل شغال');
    var phase = document.createElement('div');
    phase.className = 'cmsg';
    phase.textContent = PHASES.planning;
    c.appendChild(phase);
    var stepsEl = document.createElement('div');
    stepsEl.className = 'steps';
    c.appendChild(stepsEl);
    var activityEl = document.createElement('div');
    activityEl.className = 'activity';
    activityEl.style.display = 'none';
    c.appendChild(activityEl);
    var a = document.createElement('div');
    a.className = 'actions';
    function stopBtn() {
      a.textContent = '';
      a.appendChild(actionBtn('إيقاف', 'danger', function () {
        if (state.runId) {
          fetch('/v1/agent/runs/' + state.runId + '/cancel', {
            method: 'POST', headers: { 'Authorization': 'Bearer ' + key }
          }).catch(function () {});
        }
      }));
    }
    stopBtn();
    c.appendChild(a);
    var t0 = Date.now();

    // A7: create the run gated (auto_start:false) so it stops at the
    // plan for approval; the approval buttons replace Stop when planned.
    fetch('/v1/agent/runs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + key },
      body: JSON.stringify({ task: text, auto_start: false })
    }).then(function (r) {
      return r.json().then(function (d) { return { ok: r.ok, status: r.status, data: d }; });
    }).then(function (res) {
      if (!res.ok) {
        dropCard();
        if (res.status === 401) {
          clearKey();
          waitingCard('المفتاح مرفوض — أدخل مفتاح صحيح عشان أكمل.', function () { agentSend(text); });
          return;
        }
        errorCard((res.data && res.data.error && res.data.error.message) || ('HTTP ' + res.status));
        return;
      }
      state.runId = res.data.id;
      pollRun(key, phase, stepsEl, activityEl, a, stopBtn, t0);
    }).catch(function (err) {
      dropCard();
      errorCard('فشل الاتصال: ' + err.message);
    });
  }

  // downloadBlob fetches an authenticated URL and saves the response as
  // a file. A plain <a download> can't send the Bearer header, so the
  // download goes through fetch → blob → a temporary object URL.
  function downloadBlob(url, filename, key) {
    return fetch(url, { headers: { 'Authorization': 'Bearer ' + key } })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.blob();
      })
      .then(function (blob) {
        var u = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = u;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
        setTimeout(function () { URL.revokeObjectURL(u); }, 1000);
      });
  }

  function baseName(path) {
    var p = path.split('/');
    return p[p.length - 1] || path;
  }

  // File tree under a completed agent run. Each file has a preview
  // (click the name) and a download (⬇). "تحميل ZIP" downloads the whole
  // project. Previews: .html renders in a sandboxed iframe (no
  // same-origin, so generated markup can't touch the gateway); other
  // files show as a copyable code block.
  function addFileTree(bubble, runId, files, key) {
    var box = document.createElement('div');
    box.className = 'files';

    var head = document.createElement('div');
    head.className = 'ftitle';
    head.textContent = '📁 ملفات المشروع (' + files.length + ')';
    box.appendChild(head);

    var zipBtn = document.createElement('button');
    zipBtn.type = 'button';
    zipBtn.className = 'filebtn zipbtn';
    zipBtn.textContent = '⬇ تحميل ZIP';
    zipBtn.onclick = function () {
      zipBtn.disabled = true;
      var old = zipBtn.textContent;
      zipBtn.textContent = '… جاري التحميل';
      downloadBlob('/v1/agent/runs/' + runId + '/zip', runId + '.zip', key)
        .catch(function () {})
        .then(function () { zipBtn.disabled = false; zipBtn.textContent = old; });
    };
    box.appendChild(zipBtn);

    var preview = document.createElement('div');
    preview.className = 'preview';

    for (var i = 0; i < files.length; i++) {
      (function (path) {
        var row = document.createElement('span');
        row.className = 'filerow';

        var b = document.createElement('button');
        b.type = 'button';
        b.className = 'filebtn';
        b.textContent = path;
        b.onclick = function () {
          fetch('/v1/agent/runs/' + runId + '/files/' + path, { headers: { 'Authorization': 'Bearer ' + key } })
            .then(function (r) { return r.text(); })
            .then(function (content) {
              preview.textContent = '';
              if (/\.html?$/i.test(path)) {
                var frame = document.createElement('iframe');
                frame.setAttribute('sandbox', 'allow-scripts');
                frame.srcdoc = content;
                preview.appendChild(frame);
              } else {
                var wrap = document.createElement('div');
                wrap.className = 'code';
                var cb = document.createElement('button');
                cb.className = 'copybtn';
                cb.type = 'button';
                cb.textContent = 'نسخ';
                cb.onclick = function () { copyText(content, cb); };
                wrap.appendChild(cb);
                var pre = document.createElement('pre');
                var code = document.createElement('code');
                code.textContent = content;
                pre.appendChild(code);
                wrap.appendChild(pre);
                preview.appendChild(wrap);
              }
              log.scrollTop = log.scrollHeight;
            }).catch(function () {});
        };
        row.appendChild(b);

        // Full-tab preview for HTML: open the tab synchronously (so the
        // pop-up isn't blocked — a click gesture is required), then fill
        // it with a sandboxed iframe. sandbox has no allow-same-origin,
        // so the generated page runs in a unique origin and cannot reach
        // the gateway's origin or the API key in localStorage.
        if (/\.html?$/i.test(path)) {
          var tab = document.createElement('button');
          tab.type = 'button';
          tab.className = 'dlbtn';
          tab.title = 'فتح ' + path + ' في تاب';
          tab.textContent = '⧉';
          tab.onclick = function () {
            var win = window.open('', '_blank');
            if (win) { win.document.body.textContent = 'Loading…'; }
            fetch('/v1/agent/runs/' + runId + '/files/' + path, { headers: { 'Authorization': 'Bearer ' + key } })
              .then(function (r) { return r.text(); })
              .then(function (content) {
                if (!win) return;
                win.document.title = path;
                win.document.body.textContent = '';
                win.document.body.style.margin = '0';
                var frame = win.document.createElement('iframe');
                frame.setAttribute('sandbox', 'allow-scripts');
                frame.style.cssText = 'position:fixed;inset:0;border:0;width:100%;height:100%';
                frame.srcdoc = content;
                win.document.body.appendChild(frame);
              }).catch(function () { if (win) win.document.body.textContent = 'فشل التحميل'; });
          };
          row.appendChild(tab);
        }

        var dl = document.createElement('button');
        dl.type = 'button';
        dl.className = 'dlbtn';
        dl.title = 'تحميل ' + path;
        dl.textContent = '⬇';
        dl.onclick = function () {
          downloadBlob('/v1/agent/runs/' + runId + '/files/' + path, baseName(path), key).catch(function () {});
        };
        row.appendChild(dl);

        box.appendChild(row);
      })(files[i]);
    }
    box.appendChild(preview);
    bubble.appendChild(box);
  }

  function renderActivity(el, lines) {
    if (!lines || !lines.length) return;
    el.style.display = '';
    el.textContent = '';
    for (var i = 0; i < lines.length; i++) {
      var d = document.createElement('div');
      d.textContent = lines[i];
      el.appendChild(d);
    }
    el.scrollTop = el.scrollHeight;
  }

  function pollRun(key, phase, stepsEl, activityEl, actionsEl, stopBtn, t0) {
    var awaitingApproval = false;
    var awaitingDecision = false;
    function resume() { state.poll = setTimeout(tick, 300); }

    // A8: sensitive-command approval. Shows the command and Allow once /
    // always / Deny, posts the decision, then resumes the run.
    function showDecision(pending) {
      awaitingDecision = true;
      phase.textContent = '⚠ الوكيل عايز يشغّل أمر (' + (pending.reason || '') + '):';
      var cmd = document.createElement('div');
      cmd.className = 'activity';
      cmd.style.display = '';
      var line = document.createElement('div');
      line.textContent = '$ ' + pending.command;
      cmd.appendChild(line);
      actionsEl.textContent = '';
      actionsEl.appendChild(cmd);
      var row = document.createElement('div');
      row.className = 'actions';
      function decide(allow, remember) {
        actionsEl.textContent = '';
        stopBtn();
        fetch('/v1/agent/runs/' + state.runId + '/decision', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + key },
          body: JSON.stringify({ allow: allow, remember: remember })
        }).then(function () { awaitingDecision = false; resume(); })
          .catch(function () { errorCard('تعذّر إرسال القرار'); });
      }
      row.appendChild(actionBtn('اسمح مرة', 'primary', function () { decide(true, false); }));
      row.appendChild(actionBtn('اسمح دايمًا', '', function () { decide(true, true); }));
      row.appendChild(actionBtn('ارفض', 'danger', function () { decide(false, false); }));
      actionsEl.appendChild(row);
    }

    var pollErrors = 0;
    function tick() {
      fetch('/v1/agent/runs/' + state.runId, { headers: { 'Authorization': 'Bearer ' + key } })
        .then(function (r) { return r.json().then(function (d) { return { status: r.status, body: d }; }); })
        .then(function (res) {
          // 404: the run no longer exists — the server restarted (runs are
          // in-memory) or it was evicted. Stop cleanly instead of polling
          // a dead id forever.
          if (res.status === 404) {
            state.runId = null;
            dropCard();
            errorCard('الـrun اتفقد — السيرفر أعاد التشغيل غالبًا (الـruns مؤقتة في الذاكرة). ابدأ من جديد.');
            return;
          }
          if (res.status === 401) {
            state.runId = null; dropCard(); clearKey();
            waitingCard('المفتاح مرفوض — أدخل مفتاح صحيح.', function () {});
            return;
          }
          pollErrors = 0;
          var run = res.body;
          if (!run || !run.status) { throw new Error('bad snapshot'); }
          if (PHASES[run.status]) { phase.textContent = PHASES[run.status]; }
          if (run.steps && run.steps.length) { renderSteps(stepsEl, run.steps); }
          renderActivity(activityEl, run.log);

          // A7 plan gate: show approve/cancel and pause polling until the
          // user decides.
          if (run.status === 'planned') {
            if (!awaitingApproval) {
              awaitingApproval = true;
              actionsEl.textContent = '';
              actionsEl.appendChild(actionBtn('▶ ابدأ التنفيذ', 'primary', function () {
                actionsEl.textContent = '';
                stopBtn();
                fetch('/v1/agent/runs/' + state.runId + '/approve', {
                  method: 'POST', headers: { 'Authorization': 'Bearer ' + key }
                }).then(function () { awaitingApproval = false; resume(); })
                  .catch(function () { errorCard('تعذّر بدء التنفيذ'); });
              }));
              actionsEl.appendChild(actionBtn('إلغاء', '', function () {
                fetch('/v1/agent/runs/' + state.runId + '/cancel', {
                  method: 'POST', headers: { 'Authorization': 'Bearer ' + key }
                }).catch(function () {});
                state.runId = null;
                dropCard();
                setStatus('idle');
              }));
            }
            return; // paused: no re-poll while awaiting the user
          }

          // A8 approval gate: a sensitive command is waiting for a decision.
          if (run.status === 'awaiting_approval' && run.pending) {
            if (!awaitingDecision) { showDecision(run.pending); }
            return; // paused until the user decides
          }

          if (run.status === 'completed') {
            state.runId = null;
            var doneId = run.id;
            dropCard();
            var bubble = add('assistant', run.result || '(مفيش نتيجة)');
            if (run.files && run.files.length) { addFileTree(bubble, doneId, run.files, key); }
            var m = document.createElement('div');
            m.className = 'meta';
            m.textContent = 'agent · ' + (run.steps ? run.steps.length : 0) + ' steps · ' +
              (run.files ? run.files.length : 0) + ' files · ' + ((Date.now() - t0) / 1000).toFixed(0) + 's';
            bubble.appendChild(m);
            setStatus('done');
            return;
          }
          if (run.status === 'failed') {
            state.runId = null;
            dropCard();
            if (run.error && run.error.indexOf('اتلغى') > -1) { setStatus('idle'); }
            else { errorCard(run.error || 'فشل غير معروف'); }
            return;
          }
          log.scrollTop = log.scrollHeight;
          state.poll = setTimeout(tick, 1500);
        })
        .catch(function () {
          // Transient network/parse error — retry a few times, then give
          // up rather than spam the console forever.
          pollErrors++;
          if (pollErrors > 5) {
            state.runId = null;
            dropCard();
            errorCard('انقطع الاتصال بالـrun. جرّب من جديد.');
            return;
          }
          state.poll = setTimeout(tick, 3000);
        });
    }
    state.poll = setTimeout(tick, 1200);
  }

  // ---- Controls: Send, Stop (in the working card), Retry (in the
  // error card), Approve/Cancel (in the waiting card), New Chat.
  form.onsubmit = function (e) {
    e.preventDefault();
    if (state.status === 'working') return;
    var text = input.value.trim();
    if (!text) return;
    input.value = '';
    if (document.getElementById('agentmode').checked) { agentSend(text); }
    else { send(text); }
  };

  document.getElementById('newbtn').onclick = function () {
    if (state.ctrl) state.ctrl.abort();
    if (state.poll) { clearTimeout(state.poll); state.poll = null; }
    state.runId = null;
    msgs = [];
    log.textContent = '';
    state.card = null;
    setStatus('idle');
    input.focus();
  };

  document.getElementById('keybtn').onclick = function () { promptKey(); };

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });

  setStatus('idle');
})();
</script>
</body>
</html>
`

// handleChat serves the chat UI at exactly "/chat", GET only — the
// same shape as handleRoot for the same reasons.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use GET"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(chatHTML))
}
