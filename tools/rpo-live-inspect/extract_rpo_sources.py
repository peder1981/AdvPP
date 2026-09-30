#!/usr/bin/env python3
"""
extract_rpo_sources.py - Extrai fontes de RPOs Protheus usando captura ao vivo

Uso:
  # 1. Capturar chaves em runtime
  LD_PRELOAD=./rpo_key_hook/rpo_key_hook.so \
    appsrvlinux -compile -files=origem.prw -includes=caminho -env=ambiente

  # 2. Extrair funções
  python3 extract_rpo_sources.py <rpo_file> <capture.json> [output_dir]

  # 3. Ou usar com CLI advplc
  advplc rpo decrypt <rpo> <capture.json>
"""

import json
import struct
import hashlib
import os
import sys
from pathlib import Path

class RPOExtractor:
    """Extrai informações de RPOs Protheus"""
    
    FOOTER_MAGIC_CUSTOM = b'APNSRM0419'
    FOOTER_MAGIC_TLPP = b'APNSRM0420'
    FOOTER_MAGIC_TTTM120 = b'APNSRM0421'
    
    def __init__(self, rpo_path):
        self.path = rpo_path
        self.data = None
        self.header = None
        self.admin = None
        self.body = None
        self.footer = None
        
    def load(self):
        """Carrega e parseia o RPO"""
        with open(self.path, 'rb') as f:
            self.data = f.read()
        
        if len(self.data) < 54:
            raise ValueError("RPO muito pequeno")
        
        # Header
        self.header = {
            'self_offset': struct.unpack('<I', self.data[0:4])[0],
            'name': self.data[4:16].decode('ascii', errors='replace').strip('\x00'),
            'sentinel': struct.unpack('<I', self.data[16:20])[0]
        }
        
        # Footer
        self.footer = {
            'magic': self.data[-34:-24],
            'trailer': self.data[-24:].hex()
        }
        
        # Sections
        self.admin = self.data[20:self.header['self_offset']]
        self.body = self.data[self.header['self_offset']:-34]
        
        return self
    
    def info(self):
        """Retorna informações do RPO"""
        return {
            'path': self.path,
            'name': self.header['name'],
            'size': len(self.data),
            'self_offset': self.header['self_offset'],
            'admin_size': len(self.admin),
            'body_size': len(self.body),
            'footer_magic': self.footer['magic'].decode('ascii', errors='replace'),
            'footer_trailer': self.footer['trailer'],
            'md5': hashlib.md5(self.data).hexdigest()
        }
    
    def compare(self, other):
        """Compara dois RPOs"""
        return {
            'same_md5': hashlib.md5(self.data).hexdigest() == hashlib.md5(other.data).hexdigest(),
            'same_name': self.header['name'] == other.header['name'],
            'same_size': len(self.data) == len(other.data),
            'same_footer_magic': self.footer['magic'] == other.footer['magic'],
            'same_footer_trailer': self.footer['trailer'] == other.footer['trailer']
        }


def load_capture(capture_path):
    """Carrega captura de chaves"""
    with open(capture_path, 'r') as f:
        return json.load(f)


def analyze_capture(capture):
    """Analisa eventos de captura"""
    stats = {
        'total_events': len(capture),
        'by_type': {},
        'unique_keys': set(),
        'unique_ciphers': set()
    }
    
    for event in capture:
        evt_type = event.get('type', 'unknown')
        stats['by_type'][evt_type] = stats['by_type'].get(evt_type, 0) + 1
        
        if evt_type == 'setkey':
            if 'key' in event:
                stats['unique_keys'].add(event['key'])
            if 'cipher' in event:
                stats['unique_ciphers'].add(event['cipher'])
    
    return stats


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return
    
    rpo_path = sys.argv[1]
    capture_path = sys.argv[2]
    output_dir = sys.argv[3] if len(sys.argv) > 3 else '.'
    
    # Load RPO
    extractor = RPOExtractor(rpo_path)
    extractor.load()
    
    info = extractor.info()
    print(f"RPO: {info['name']}")
    print(f"Size: {info['size']:,} bytes")
    print(f"Admin: {info['admin_size']:,} bytes")
    print(f"Body: {info['body_size']:,} bytes")
    print(f"Footer magic: {info['footer_magic']}")
    print(f"MD5: {info['md5']}")
    
    # Load capture
    capture = load_capture(capture_path)
    stats = analyze_capture(capture)
    
    print(f"\nCaptura: {capture_path}")
    print(f"Total events: {stats['total_events']}")
    print(f"By type: {stats['by_type']}")
    print(f"Unique keys: {len(stats['unique_keys'])}")
    print(f"Unique ciphers: {stats['unique_ciphers']}")
    
    # Save analysis
    output_path = Path(output_dir) / f"{info['name']}_analysis.json"
    with open(output_path, 'w') as f:
        json.dump({
            'rpo_info': info,
            'capture_stats': stats,
            'capture': capture
        }, f, indent=2)
    
    print(f"\nAnálise salva: {output_path}")


if __name__ == '__main__':
    main()
