import { StreamLanguage, StringStream } from '@codemirror/language';

// Palavras-chave do lexer do AdvPP (pkg/lexer/tokens.go), sem caixa.
const PALAVRAS = new Set(('array as begin break case catch character class constructor continue data date ' +
  'decimal default do double else elseif end endcase endclass enddo endif endinterface endtry exit finally ' +
  'fixed for from function global if implements integer interface json local logical loop main memo method ' +
  'next nil numeric object operator otherwise parameter parameters private procedure protected public ' +
  'recover return self sequence static step throw to try undefined user using variadic variant while ' +
  'namespace wsrestful wsmethod').split(' '));

interface Estado { comentario: boolean }

// Realce de AdvPL/TLPP para o CodeMirror 6 (modo de fluxo, linha a linha).
export const advpl = StreamLanguage.define<Estado>({
  name: 'advpl',
  startState: () => ({ comentario: false }),
  token(s: StringStream, e: Estado): string | null {
    if (e.comentario) {
      if (s.skipTo('*/')) { s.next(); s.next(); e.comentario = false; } else { s.skipToEnd(); }
      return 'comment';
    }
    if (s.eatSpace()) { return null; }
    if (s.match('//')) { s.skipToEnd(); return 'comment'; }
    if (s.match('/*')) { e.comentario = true; return 'comment'; }
    if (s.sol() && s.match(/^\s*#\s*[a-zA-Z]+/)) { return 'meta'; }
    const c = s.peek();
    if (c === '"' || c === "'") {
      s.next();
      while (!s.eol()) { if (s.next() === c) { break; } }
      return 'string';
    }
    if (s.match(/^\.(t|f|and|or|not)\./i)) { return 'atom'; }
    if (s.match(/^\d+(\.\d+)?/)) { return 'number'; }
    if (s.match(/^[A-Za-z_][A-Za-z0-9_]*/)) {
      const p = s.current().toLowerCase();
      if (PALAVRAS.has(p)) { return 'keyword'; }
      return s.peek() === '(' ? 'variableName.function' : 'variableName';
    }
    if (s.match(/^(:=|==|!=|<>|<=|>=|\+\+|--|\+=|-=|->|::|[-+*/%^=<>!$@&|:])/)) { return 'operator'; }
    s.next();
    return null;
  },
  languageData: { commentTokens: { line: '//', block: { open: '/*', close: '*/' } } },
});
