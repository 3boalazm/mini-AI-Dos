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
// Beyond plain chat it surfaces the request telemetry the gateway
// already exposes — model, token usage, duration, request id, and the
// X-RateLimit-* headers — and renders fenced code blocks with a copy
// button. No template engine and no JS backticks: this constant is a
// Go raw string, so backticks cannot appear anywhere in the page.
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
  header { padding: .8rem 1rem; border-bottom: 1px solid rgba(127,127,127,.25); display: flex; justify-content: space-between; align-items: center; gap: .6rem; }
  header h1 { font-size: 1.05rem; margin: 0; }
  #prov { font-size: .72rem; opacity: .65; direction: ltr; }
  header button { font-size: .8rem; padding: .3rem .7rem; border-radius: 6px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; cursor: pointer; }
  #log { flex: 1; overflow-y: auto; padding: 1rem; display: flex; flex-direction: column; gap: .6rem; }
  .msg { max-width: 46rem; padding: .6rem .9rem; border-radius: 12px; line-height: 1.6; }
  .msg .txt { white-space: pre-wrap; }
  .user { align-self: flex-start; background: rgba(59,130,246,.18); }
  .assistant { align-self: flex-end; background: rgba(127,127,127,.15); }
  .err { align-self: center; color: #d33; font-size: .85rem; }
  .code { position: relative; direction: ltr; text-align: left; margin: .45rem 0; }
  .code pre { overflow-x: auto; background: rgba(0,0,0,.25); padding: .7rem .8rem; padding-top: 1.6rem; border-radius: 8px; margin: 0; font-size: .85rem; }
  @media (prefers-color-scheme: light) { .code pre { background: rgba(0,0,0,.07); } }
  .code .lang { position: absolute; top: .3rem; left: .6rem; font-size: .68rem; opacity: .55; text-transform: lowercase; }
  .copybtn { position: absolute; top: .25rem; right: .35rem; font-size: .7rem; padding: .15rem .55rem; border-radius: 5px; border: 1px solid rgba(127,127,127,.4); background: rgba(127,127,127,.15); color: inherit; cursor: pointer; }
  .meta { font-size: .7rem; opacity: .6; margin-top: .4rem; direction: ltr; text-align: left; }
  form { display: flex; gap: .5rem; padding: .8rem 1rem; border-top: 1px solid rgba(127,127,127,.25); }
  textarea { flex: 1; resize: none; padding: .6rem .8rem; border-radius: 10px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; font: inherit; height: 3rem; }
  form button { padding: 0 1.2rem; border-radius: 10px; border: 0; background: #3b82f6; color: #fff; font: inherit; cursor: pointer; }
  form button:disabled { opacity: .5; }
</style>
</head>
<body>
<header>
  <h1>Mini AI-DOS — شات</h1>
  <span id="prov"></span>
  <button id="keybtn" type="button">تغيير المفتاح</button>
</header>
<div id="log"></div>
<form id="f">
  <textarea id="in" placeholder="اكتب رسالتك… (Enter للإرسال، Shift+Enter لسطر جديد)"></textarea>
  <button id="send" type="submit">إرسال</button>
</form>
<script>
(function () {
  var KEY = 'aidos_api_key';
  var log = document.getElementById('log');
  var form = document.getElementById('f');
  var input = document.getElementById('in');
  var sendBtn = document.getElementById('send');
  var msgs = [];
  var memKey = '';

  // Strict privacy modes make localStorage throw; fall back to an
  // in-memory key so the page still works for the session.
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
  function getKey(forceAsk) {
    var k = loadKey();
    if (!k || forceAsk) {
      k = (window.prompt('حط الـ API key بتاع الـ gateway (بيتخزن في متصفحك بس):') || '').trim();
      if (k) storeKey(k);
    }
    return k;
  }

  // Active provider in the header — the same fact /health reports.
  fetch('/health').then(function (r) { return r.json(); }).then(function (h) {
    if (h && h.provider) { document.getElementById('prov').textContent = 'provider: ' + h.provider; }
  }).catch(function () {});

  function copyText(txt, btn) {
    function done() {
      var old = btn.textContent;
      btn.textContent = 'اتنسخ ✓';
      setTimeout(function () { btn.textContent = old; }, 1500);
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).then(done).catch(function () { fallback(); });
    } else { fallback(); }
    function fallback() {
      var ta = document.createElement('textarea');
      ta.value = txt;
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); done(); } catch (e) {}
      ta.remove();
    }
  }

  // Renders message text into el, turning fenced code blocks into
  // copyable ones. Built entirely with textContent — nothing the model
  // returns is ever parsed as HTML. The fence marker is built from
  // char codes because this whole page lives in a Go raw string,
  // where a literal backtick would end the string.
  var FENCE = String.fromCharCode(96, 96, 96);
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
      d.dir = 'auto';
      var t = document.createElement('div');
      t.className = 'txt';
      t.textContent = text;
      d.appendChild(t);
    }
    log.appendChild(d);
    log.scrollTop = log.scrollHeight;
    return d;
  }

  // The per-reply telemetry line: model, tokens, duration, rate-limit
  // budget, request id — everything the gateway reports about the call.
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

  document.getElementById('keybtn').onclick = function () { getKey(true); };

  form.onsubmit = function (e) {
    e.preventDefault();
    var text = input.value.trim();
    if (!text) return;
    var key = getKey(false);
    if (!key) { add('err', 'محتاج API key الأول — دوس "تغيير المفتاح"'); return; }
    input.value = '';
    sendBtn.disabled = true;
    msgs.push({ role: 'user', content: text });
    add('user', text);
    var pending = add('assistant', '…');
    var t0 = Date.now();
    fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + key },
      body: JSON.stringify({ messages: msgs })
    }).then(function (r) {
      return r.json().then(function (data) { return { ok: r.ok, status: r.status, resp: r, data: data }; });
    }).then(function (res) {
      pending.remove();
      if (!res.ok) {
        msgs.pop();
        var m = (res.data && res.data.error && res.data.error.message) || ('HTTP ' + res.status);
        add('err', m);
        if (res.status === 401) { clearKey(); }
        return;
      }
      var reply = res.data.choices[0].message;
      msgs.push(reply);
      var bubble = add('assistant', reply.content);
      addMeta(bubble, res.data, res.resp, Date.now() - t0);
      log.scrollTop = log.scrollHeight;
    }).catch(function (err) {
      msgs.pop();
      pending.remove();
      add('err', 'فشل الاتصال: ' + err.message);
    }).finally(function () {
      sendBtn.disabled = false;
      input.focus();
    });
  };

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });
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
