#!/usr/bin/env python3
"""
Ferramenta completa de desmontagem de RPOs Protheus.
Extrai informações sem necessidade de descriptografia.
"""

import sys
import struct
import hashlib
import re
from collections import Counter, defaultdict
from pathlib import Path
import math

class RPODismantler:
    """Desmonta RPOs Protheus extraindo informações estruturais."""
    
    # Magic bytes conhecidos
    MAGIC = {
        b'\x56\xbd\xb6\x00': 'custom',
        b'\x80\x91\x3f\x00': 'tlpp',
        b'\xc7\xb9\xf0\x00': 'tttm120_patch',
        b'\xa9\xb3\x6c\x16': 'tttm120_original',
    }
    
    # Footer magics
    FOOTER_MAGIC = {
        b'APNSRM0419': 'custom',
        b'APNSRM0420': 'tlpp',
        b'APNSRM0421': 'tttm120',
    }
    
    def __init__(self, rpo_path):
        self.path = Path(rpo_path)
        self.data = self.path.read_bytes()
        self.info = {}
        self.functions = []
        self.routines = []
        self.strings = []
        self.apo_candidates = []
        self.repeated_blocks = []
        
    def dismantle(self):
        """Realizar desmontagem completa."""
        self._extract_header()
        self._extract_footer()
        self._analyze_entropy()
        self._find_repeated_blocks()
        self._find_apo_candidates()
        self._extract_strings()
        self._identify_functions()
        return self._generate_report()
    
    def _extract_header(self):
        """Extrair informações do header."""
        if len(self.data) < 28:
            return
            
        magic = self.data[:4]
        self.info['magic'] = magic.hex()
        self.info['magic_type'] = self.MAGIC.get(magic, 'unknown')
        self.info['name'] = self.data[4:12].decode('ascii', errors='replace').strip('\x00')
        self.info['sentinel'] = struct.unpack('<I', self.data[12:16])[0]
        self.info['field_16_20'] = struct.unpack('<I', self.data[16:20])[0]
        self.info['flags'] = struct.unpack('<I', self.data[20:24])[0]
        self.info['size_field'] = struct.unpack('<I', self.data[24:28])[0]
        self.info['size'] = len(self.data)
        self.info['md5'] = hashlib.md5(self.data).hexdigest()
        
    def _extract_footer(self):
        """Extrair informações do footer."""
        for magic, type_name in self.FOOTER_MAGIC.items():
            pos = self.data.rfind(magic)
            if pos > 0 and pos < len(self.data) - 36:
                self.info['footer_offset'] = pos
                self.info['footer_magic'] = magic
                self.info['footer_type'] = type_name
                self.info['trailer_sha1'] = self.data[pos+12:pos+36].hex()
                self.info['content_size'] = pos - 12
                return
                
    def _analyze_entropy(self):
        """Analisar entropia por regiões."""
        region_size = 1024 * 1024  # 1MB
        regions = []
        
        for i in range(0, len(self.data), region_size):
            region = self.data[i:i+region_size]
            if len(region) == 0:
                break
                
            # Calcular entropia
            counts = Counter(region)
            entropy = 0.0
            for count in counts.values():
                p = count / len(region)
                if p > 0:
                    entropy -= p * math.log2(p)
            
            regions.append({
                'offset': i,
                'entropy': entropy,
                'zeros': region.count(0),
                'unique_bytes': len(set(region)),
            })
            
        self.info['regions'] = regions
        self.info['avg_entropy'] = sum(r['entropy'] for r in regions) / len(regions) if regions else 0
        
    def _find_repeated_blocks(self):
        """Encontrar blocos repetidos."""
        block_size = 32
        blocks = defaultdict(list)
        
        for i in range(0, len(self.data) - block_size, 8):
            block = self.data[i:i+block_size]
            if block != b'\x00' * block_size:
                block_hash = hashlib.md5(block).hexdigest()[:8]
                blocks[block_hash].append(i)
        
        repeated = {k: v for k, v in blocks.items() if len(v) > 1}
        self.repeated_blocks = sorted(repeated.items(), key=lambda x: -len(x[1]))[:20]
        
    def _find_apo_candidates(self):
        """Encontrar candidatos a estruturas APO."""
        for i in range(0, min(len(self.data), 100000), 4):
            try:
                size = struct.unpack('<I', self.data[i:i+4])[0]
                if 50 < size < 50000:
                    if i + 4 < len(self.data):
                        type_byte = self.data[i+4]
                        if 0x00 <= type_byte <= 0x20:
                            self.apo_candidates.append({
                                'offset': i,
                                'size': size,
                                'type': type_byte,
                                'preview': self.data[i:i+16].hex()
                            })
            except:
                continue
                
    def _extract_strings(self):
        """Extrair strings significativas."""
        raw_strings = re.findall(rb'[\x20-\x7e]{6,}', self.data)
        
        for s in raw_strings:
            try:
                s_str = s.decode('ascii', errors='replace')
                if any(c < ' ' or c > '~' for c in s_str):
                    continue
                if re.search(r'[A-Z]{3,}|[A-Z]_[A-Z]|\.(prw|tlpp|prg)', s_str):
                    self.strings.append(s_str)
            except:
                pass
                
    def _identify_functions(self):
        """Identificar funções e rotinas."""
        # User functions
        u_funcs = re.findall(rb'U_[A-Z0-9_]{3,10}', self.data)
        self.functions = sorted(set(f.decode('ascii', errors='replace') for f in u_funcs))
        
        # Routines (pattern: XXX999)
        routines = re.findall(rb'[A-Z]{2,4}[0-9]{3,5}', self.data)
        self.routines = sorted(set(r.decode('ascii', errors='replace') for r in routines))
        
    def _generate_report(self):
        """Gerar relatório completo."""
        report = []
        report.append("=" * 80)
        report.append(f"RPO DISMANTLE REPORT: {self.path.name}")
        report.append("=" * 80)
        report.append("")
        
        # Basic info
        report.append("[BASIC INFORMATION]")
        report.append(f"  File: {self.path}")
        report.append(f"  Size: {self.info.get('size', 0):,} bytes")
        report.append(f"  MD5: {self.info.get('md5', 'N/A')}")
        report.append(f"  Magic: {self.info.get('magic', 'N/A')}")
        report.append(f"  Type: {self.info.get('magic_type', 'N/A')}")
        report.append(f"  Name: {self.info.get('name', 'N/A')}")
        report.append(f"  Entropy: {self.info.get('avg_entropy', 0):.6f} bits/byte")
        report.append("")
        
        # Header
        report.append("[HEADER]")
        for key in ['sentinel', 'field_16_20', 'flags', 'size_field']:
            if key in self.info:
                report.append(f"  {key}: 0x{self.info[key]:08X}")
        report.append("")
        
        # Footer
        if 'footer_offset' in self.info:
            report.append("[FOOTER]")
            report.append(f"  Offset: {self.info['footer_offset']}")
            report.append(f"  Magic: {self.info['footer_magic']}")
            report.append(f"  SHA-1: {self.info['trailer_sha1']}")
            report.append("")
        
        # Functions
        report.append(f"[FUNCTIONS] ({len(self.functions)} found)")
        for func in self.functions[:50]:
            report.append(f"  U_{func}" if not func.startswith('U_') else f"  {func}")
        if len(self.functions) > 50:
            report.append(f"  ... and {len(self.functions) - 50} more")
        report.append("")
        
        # Routines
        report.append(f"[ROUTINES] ({len(self.routines)} found)")
        for routine in self.routines[:50]:
            report.append(f"  {routine}")
        if len(self.routines) > 50:
            report.append(f"  ... and {len(self.routines) - 50} more")
        report.append("")
        
        # Repeated blocks
        if self.repeated_blocks:
            report.append(f"[REPEATED BLOCKS] (top {len(self.repeated_blocks)})")
            for i, (hash_val, offsets) in enumerate(self.repeated_blocks[:10]):
                report.append(f"  [{i+1}] {len(offsets)}x - offsets: {offsets[:5]}")
            report.append("")
        
        # APO candidates
        if self.apo_candidates:
            report.append(f"[APO CANDIDATES] ({len(self.apo_candidates)} found)")
            for cand in sorted(self.apo_candidates, key=lambda x: -x['size'])[:10]:
                report.append(f"  Offset {cand['offset']:8d}: size={cand['size']:5d}, type=0x{cand['type']:02x}")
            report.append("")
        
        # Strings
        report.append(f"[STRINGS] ({len(self.strings)} found)")
        for s in sorted(set(self.strings))[:30]:
            report.append(f"  {s}")
        report.append("")
        
        return "\n".join(report)


def main():
    if len(sys.argv) < 2:
        print("Usage: python dismantle_rpo.py <rpo_file> [output_file]")
        sys.exit(1)
        
    rpo_path = sys.argv[1]
    output_path = sys.argv[2] if len(sys.argv) > 2 else None
    
    dismantler = RPODismantler(rpo_path)
    report = dismantler.dismantle()
    
    print(report)
    
    if output_path:
        Path(output_path).write_text(report)
        print(f"\nReport saved to: {output_path}")


if __name__ == "__main__":
    main()
