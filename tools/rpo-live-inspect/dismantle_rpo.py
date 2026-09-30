#!/usr/bin/env python3
"""
Desmontagem HONESTA de RPOs Protheus.

Histórico: a versão anterior deste script aplicava regex de "funções" (U_...)
e "rotinas" (XXX999) diretamente sobre TODO o conteúdo do RPO. Como o
conteúdo é cifra+zlib (entropia ~8.0), os nomes reportados (AP448, DK158,
U_4SH, ...) eram FALSOS POSITIVOS de regex sobre ruído. Ver:
  - pkg/rpo/forensics.go        (classificador de regiões)
  - pkg/rpo/forensics_test.go   (prova estatística dos falsos positivos)
  - docs/RPO-GROUND-TRUTH.md    (veredito documentado)

Esta versão:
  1. Classifica cada janela por entropia de Shannon.
  2. Só extrai strings/nomes de janelas PLAINTEXT (baixa entropia).
  3. Se não houver janela plaintext, reporta explicitamente que NÃO há
     estrutura legível — em vez de inventar nomes.
"""

import sys
import struct
import hashlib
import re
import math
from collections import Counter, defaultdict
from pathlib import Path

WINDOW = 4096
PLAINTEXT_ENTROPY_MAX = 4.5   # mesma constante de pkg/rpo/forensics.go
CIPHERTEXT_ENTROPY_MIN = 7.5


def shannon_entropy(data: bytes) -> float:
    if not data:
        return 0.0
    counts = Counter(data)
    total = len(data)
    ent = 0.0
    for c in counts.values():
        p = c / total
        ent -= p * math.log2(p)
    return ent


class RPODismantler:
    MAGIC = {
        b'\x56\xbd\xb6\x00': 'custom',
        b'\x80\x91\x3f\x00': 'tlpp',
        b'\xc7\xb9\xf0\x00': 'tttm120_patch',
        b'\xa9\xb3\x6c\x16': 'tttm120_original',
    }
    FOOTER_MAGIC = {
        b'APNSRM0419': 'custom',
        b'APNSRM0420': 'tlpp',
        b'APNSRM0421': 'tttm120',
    }

    def __init__(self, rpo_path):
        self.path = Path(rpo_path)
        self.data = self.path.read_bytes()
        self.info = {}
        self.regions = []

    def dismantle(self):
        self._extract_header()
        self._extract_footer()
        self._classify_regions()
        return self._generate_report()

    def _extract_header(self):
        if len(self.data) < 28:
            return
        self.info['magic'] = self.data[:4].hex()
        self.info['magic_type'] = self.MAGIC.get(self.data[:4], 'unknown')
        self.info['name'] = self.data[4:12].decode('ascii', errors='replace').strip('\x00')
        self.info['sentinel'] = struct.unpack('<I', self.data[12:16])[0]
        self.info['size'] = len(self.data)
        self.info['md5'] = hashlib.md5(self.data).hexdigest()

    def _extract_footer(self):
        for magic, type_name in self.FOOTER_MAGIC.items():
            pos = self.data.rfind(magic)
            if 0 < pos < len(self.data) - 36:
                self.info['footer_offset'] = pos
                self.info['footer_magic'] = magic.decode()
                self.info['footer_type'] = type_name
                self.info['content_size'] = pos - 8
                return

    def _classify_regions(self):
        for off in range(0, len(self.data), WINDOW):
            win = self.data[off:off + WINDOW]
            if not win:
                break
            ent = shannon_entropy(win)
            if all(b == 0 for b in win):
                kind = 'zero'
            elif ent >= CIPHERTEXT_ENTROPY_MIN:
                kind = 'ciphertext'
            elif ent <= PLAINTEXT_ENTROPY_MAX:
                kind = 'plaintext'
            else:
                kind = 'mixed'
            self.regions.append({'offset': off, 'size': len(win), 'entropy': ent, 'kind': kind})

    def _plaintext_strings(self, min_len=6):
        """Extrai strings SOMENTE de janelas plaintext."""
        found = []
        for r in self.regions:
            if r['kind'] != 'plaintext':
                continue
            win = self.data[r['offset']:r['offset'] + r['size']]
            for m in re.finditer(rb'[\x20-\x7e]{%d,}' % min_len, win):
                found.append((r['offset'] + m.start(), m.group().decode('ascii', 'replace')))
        return found

    def _generate_report(self):
        L = []
        L.append("=" * 78)
        L.append(f"RPO DISMANTLE (honesto) — {self.path.name}")
        L.append("=" * 78)
        L.append("")
        L.append("[INFORMAÇÃO BÁSICA]")
        L.append(f"  Arquivo : {self.path}")
        L.append(f"  Tamanho : {self.info.get('size', 0):,} bytes")
        L.append(f"  MD5     : {self.info.get('md5', 'N/A')}")
        L.append(f"  Magic   : {self.info.get('magic', 'N/A')} ({self.info.get('magic_type', '?')})")
        L.append(f"  Nome    : {self.info.get('name', 'N/A')}")
        if 'footer_offset' in self.info:
            L.append(f"  Footer  : {self.info['footer_magic']} @ {self.info['footer_offset']}")
        L.append("")

        # Resumo de regiões
        kinds = Counter(r['kind'] for r in self.regions)
        total = sum(r['size'] for r in self.regions) or 1
        mean_ent = sum(r['entropy'] * r['size'] for r in self.regions) / total
        L.append("[CLASSIFICAÇÃO DE CONTEÚDO]")
        L.append(f"  Entropia média   : {mean_ent:.4f} bits/byte")
        for k in ('zero', 'ciphertext', 'mixed', 'plaintext'):
            b = sum(r['size'] for r in self.regions if r['kind'] == k)
            L.append(f"  {k:11s}: {b:>12,} bytes ({100*b/total:6.2f}%)")
        L.append("")

        # Veredito explícito sobre extração de nomes
        n_plain = kinds.get('plaintext', 0)
        strings = self._plaintext_strings()
        L.append("[EXTRAÇÃO DE NOMES/STRINGS]")
        if n_plain == 0 or not strings:
            L.append("  Nenhuma janela plaintext com strings.")
            L.append("  VEREDITO: o conteúdo é integralmente cifra/comprimido;")
            L.append("  NÃO há estrutura legível. Qualquer nome aqui seria ruído.")
            L.append("  (Regex aplicada a este arquivo reproduz exatamente os")
            L.append("   'falsos nomes' de relatórios antigos — ver")
            L.append("   docs/RPO-GROUND-TRUTH.md.)")
        else:
            L.append(f"  {len(strings)} string(s) em {n_plain} janela(s) plaintext:")
            for off, s in strings[:60]:
                L.append(f"    +{off:>10}  {s}")
        L.append("")
        L.append("NOTA: para decodificar o conteúdo cifrado é necessária uma captura")
        L.append("      de chave ao vivo da MESMA sessão de compilação:")
        L.append("      `advplc rpo decrypt <rpo> <captura.json>`.")
        return "\n".join(L)


def main():
    if len(sys.argv) < 2:
        print("Usage: python dismantle_rpo.py <rpo_file> [output_file]")
        sys.exit(1)
    d = RPODismantler(sys.argv[1])
    report = d.dismantle()
    print(report)
    if len(sys.argv) > 2:
        Path(sys.argv[2]).write_text(report)
        print(f"\nReport saved to: {sys.argv[2]}")


if __name__ == "__main__":
    main()
