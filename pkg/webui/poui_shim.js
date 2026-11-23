// Shim da ponte JS <-> AdvPL para apps PO-UI (@totvs/protheus-lib-core).
// Injetado inline no index.html servido pelo advplc serve — o arquivo do
// app em disco/zip nunca é alterado. Sem build, JS puro.
(function () {
  var q = new URLSearchParams(location.search);
  var m = document.cookie.match(/(?:^|;\s*)poui_sid=([^;]+)/);
  var sid = q.get('sid') || (m ? decodeURIComponent(m[1]) : '');
  var app = q.get('app') || '';

  // Contexto da sessão: chaves de sessionStorage que a lib espera.
  try {
    var x = new XMLHttpRequest();
    x.open('GET', '/poui-ctx?sid=' + encodeURIComponent(sid), false);
    x.send(null);
    if (x.status === 200) {
      var ctx = JSON.parse(x.responseText);
      for (var k in ctx) { if (ctx.hasOwnProperty(k)) sessionStorage.setItem(k, ctx[k]); }
    }
  } catch (e) {}

  var tc = window.twebchannel = window.twebchannel || {};
  tc.gotConnection = false;
  tc.eventTarget = tc.eventTarget || new EventTarget();
  tc.jsToAdvpl = function (type, content) {
    fetch('/poui-ui?sid=' + encodeURIComponent(sid), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ app: app, type: type, content: content })
    }).catch(function () {});
  };

  // O servidor emite tudo como evento SSE "message" (só data:); o tipo
  // real vem no JSON: {type:"advpltojs", data:{type, content}}.
  var es = new EventSource('/events?s=' + encodeURIComponent(sid) + '&canal=poui');
  es.onopen = function () { tc.gotConnection = true; };
  es.onmessage = function (m) {
    var d;
    try { d = JSON.parse(m.data); } catch (e) { return; }
    if (!d || d.type !== 'advpltojs' || !d.data) { return; }
    var p = d.data;
    if (p.type === 'close') { window.close(); return; }
    var ev = new Event(p.type || 'message');
    ev.protheusResponse = p.content;
    ev.subscriber = tc[p.type] && tc[p.type].subscriber;
    tc.eventTarget.dispatchEvent(ev);
  };
})();
