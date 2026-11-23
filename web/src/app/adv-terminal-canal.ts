// Saída dos terminais (op "term"), por id. Fica fora de adv-terminal.ts
// para o app.ts não puxar o xterm.js para o pacote inicial: o
// AdvTerminal é carregado sob demanda (@defer). Bytes que chegam antes de
// o componente existir ficam guardados e são escritos quando ele registra.
const escritores = new Map<number, (d: Uint8Array) => void>();
const pendentes = new Map<number, Uint8Array[]>();

export function entregaTerm(id: number, dados: Uint8Array) {
  const w = escritores.get(id);
  if (w) { w(dados); return; }
  pendentes.set(id, [...(pendentes.get(id) ?? []), dados]);
}

// registraTerm liga o escritor do terminal id e já escreve o que estava
// pendente.
export function registraTerm(id: number, w: (d: Uint8Array) => void) {
  escritores.set(id, w);
  for (const d of pendentes.get(id) ?? []) { w(d); }
  pendentes.delete(id);
}

export function removeTerm(id: number) {
  escritores.delete(id);
}
