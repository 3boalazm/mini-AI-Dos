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
const chatHTML = `<!doctype html>
<html lang="ar" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mini AI-DOS — شات</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; margin: 0; display: flex; flex-direction: column; height: 100dvh; }
  header { padding: .8rem 1rem; border-bottom: 1px solid rgba(127,127,127,.25); display: flex; justify-content: space-between; align-items: center; }
  header h1 { font-size: 1.05rem; margin: 0; }
  header button { font-size: .8rem; padding: .3rem .7rem; border-radius: 6px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; cursor: pointer; }
  #log { flex: 1; overflow-y: auto; padding: 1rem; display: flex; flex-direction: column; gap: .6rem; }
  .msg { max-width: 46rem; padding: .6rem .9rem; border-radius: 12px; white-space: pre-wrap; line-height: 1.6; }
  .user { align-self: flex-start; background: rgba(59,130,246,.18); }
  .assistant { align-self: flex-end; background: rgba(127,127,127,.15); }
  .err { align-self: center; color: #d33; font-size: .85rem; }
  form { display: flex; gap: .5rem; padding: .8rem 1rem; border-top: 1px solid rgba(127,127,127,.25); }
  textarea { flex: 1; resize: none; padding: .6rem .8rem; border-radius: 10px; border: 1px solid rgba(127,127,127,.4); background: transparent; color: inherit; font: inherit; height: 3rem; }
  form button { padding: 0 1.2rem; border-radius: 10px; border: 0; background: #3b82f6; color: #fff; font: inherit; cursor: pointer; }
  form button:disabled { opacity: .5; }
</style>
</head>
<body>
<header>
  <h1>Mini AI-DOS — شات</h1>
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

  function add(role, text) {
    var d = document.createElement('div');
    d.className = 'msg ' + role;
    d.dir = 'auto';
    d.textContent = text;
    log.appendChild(d);
    log.scrollTop = log.scrollHeight;
    return d;
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
    fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + key },
      body: JSON.stringify({ messages: msgs })
    }).then(function (r) {
      return r.json().then(function (data) { return { ok: r.ok, status: r.status, data: data }; });
    }).then(function (res) {
      if (!res.ok) {
        msgs.pop();
        pending.remove();
        var m = (res.data && res.data.error && res.data.error.message) || ('HTTP ' + res.status);
        add('err', m);
        if (res.status === 401) { clearKey(); }
        return;
      }
      var reply = res.data.choices[0].message;
      msgs.push(reply);
      pending.textContent = reply.content;
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
