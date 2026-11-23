// Round-trip síncrono dos hooks OnBeforeSave/OnBeforeRemove (Onda 3a.6).
// O frontend posta {reqId, data} como evento "beforesave"/"beforeremove";
// a VM responde com a op "beforeResult" {reqId, ok} que o app.ts resolve
// aqui. Timeout de 30 s equivale a recusar (ok=false), como no plano.

export interface DynPost {
  (id: number, event: string, value: unknown): void;
}

const pendentes = new Map<number, (ok: boolean) => void>();
let proximoReq = 1;

export function dynAntes(post: DynPost, id: number, ev: 'beforesave' | 'beforeremove', data: unknown): Promise<boolean> {
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
