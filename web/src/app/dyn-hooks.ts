// Round-trip síncrono dos hooks OnBeforeSave/OnBeforeRemove (Onda 3a.6).
// O frontend posta {reqId, data} como evento "beforesave"/"beforeremove";
// a VM responde com a op "beforeResult" {reqId, ok} que o app.ts resolve
// aqui. Timeout de 30 s equivale a recusar (ok=false), como no plano.

import { HttpInterceptorFn, HttpResponse } from '@angular/common/http';
import { from, map } from 'rxjs';

export interface DynPost {
  (id: number, event: string, value: unknown): void;
}

const pendentes = new Map<number, (ok: boolean) => void>();
let proximoReq = 1;

export function dynAntes(post: DynPost, id: number, ev: string, data: unknown): Promise<boolean> {
  const reqId = proximoReq++;
  return new Promise<boolean>((resolve) => {
    const timer = setTimeout(() => {
      pendentes.delete(reqId);
      resolve(false); // timeout = recusa
    }, 30000);
    pendentes.set(reqId, (ok) => {
      clearTimeout(timer);
      resolve(ok);
    });
    post(id, ev, { reqId, data });
  });
}

// chamado pelo app.ts ao receber a op {op:"beforeResult", reqId, ok}
export function dynAntesResultado(reqId: number, ok: boolean) {
  const r = pendentes.get(reqId);
  if (r) {
    pendentes.delete(reqId);
    r(!!ok);
  }
}

// ---- Templates reais do PO-UI (@po-ui/ng-templates) ----
// Os "before" dos templates (beforeSave, beforeRemove...) aceitam uma URL:
// o PO-UI faz POST em <url>[/<id>] com o registro e lê {allowAction}. A
// URL reservada abaixo nunca sai do navegador: o interceptador faz o
// round-trip ao AdvPL (dynAntes) e responde {allowAction: ok}.
let poster: DynPost | null = null;

// app.ts registra o post da sessão (POST /ui) ao iniciar.
export function defineDynPoster(p: DynPost) {
  poster = p;
}

const ANTES = '/__advpp/antes/';

// antesUrl: URL do hook <ev> do componente <id> (ex.: beforesave).
export function antesUrl(id: number, ev: string): string {
  return `${ANTES}${id}/${ev}`;
}

export const advppAntesInterceptor: HttpInterceptorFn = (req, next) => {
  const i = req.url.indexOf(ANTES);
  if (i < 0 || !poster) {
    return next(req);
  }
  const [id, ev] = req.url.slice(i + ANTES.length).split('/');
  return from(dynAntes(poster, Number(id), ev, req.body)).pipe(
    map(ok => new HttpResponse({ status: 200, body: { allowAction: ok } })),
  );
};
