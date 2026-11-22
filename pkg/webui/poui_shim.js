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

  var es = new EventSource('/events?s=' + encodeURIComponent(sid) + '&canal=poui');
  es.onopen = function () { tc.gotConnection = true; };
  es.addEventListener('advpltojs', function (ev) {
    var d = JSON.parse(ev.data);
    var e = new Event(d.type || 'message');
    e.protheusResponse = d.content;
    e.subscriber = tc[d.type] && tc[d.type].subscriber;
    tc.eventTarget.dispatchEvent(e);
  });
})();
