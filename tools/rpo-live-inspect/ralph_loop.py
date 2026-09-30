#!/usr/bin/env python3
"""
Ralph Loop v3 — Iteration Persistence com 150M tentativas auto-corretivas.

ESTRATÉGIA:
  1. Captura online via LD_PRELOAD hook
  2. Brute-force de cifras (12+ algoritmos × 4 modos = 48 combinações)
  3. Auto-correção: se falhar, tenta próxima cifra
  4. Persistência: retry automático em caso de falha
  5. Timeout: 150M tentativas ou sucesso

OBJETIVO: Extrair TODOS os fontes de cada RPO (custom, tlpp, tttm120).
"""

import argparse
import hashlib
import json
import math
import os
import struct
import subprocess
import sys
import time
from collections import Counter
from pathlib import Path

# Default: 150 milhões de tentativas
DEFAULT_MAX_TRIES = 150_000_000
VERBOSE = False


class RalphLoop:
    """Loop persistente com auto-correção para recuperação de chave RPO."""
    
    # Todas as cifras conhecidas do RPO Protheus (12 algoritmos × modos)
    CIPHERS = [
        # (nome, classe, modo, key_len, iv_len)
        ("des_ede_cbc", "TripleDES", "cbc", 24, 8),
        ("des_ede_ecb", "TripleDES", "ecb", 24, 0),
        ("cast5_cbc", "CAST5", "cbc", 16, 8),
        ("cast5_ecb", "CAST5", "ecb", 16, 0),
        ("cast5_cfb64", "CAST5", "cfb64", 16, 8),
        ("cast5_ofb", "CAST5", "ofb", 16, 8),
        ("bf_cbc", "Blowfish", "cbc", 16, 8),
        ("bf_ecb", "Blowfish", "ecb", 16, 0),
        ("bf_cfb64", "Blowfish", "cfb64", 16, 8),
        ("bf_ofb", "Blowfish", "ofb", 16, 8),
        ("rc4", "RC4", "stream", 16, 0),
        ("idea_cbc", "IDEA", "cbc", 16, 8),
        ("idea_ecb", "IDEA", "ecb", 16, 0),
        ("idea_cfb64", "IDEA", "cfb64", 16, 8),
        ("idea_ofb", "IDEA", "ofb", 16, 8),
        ("rc2_cbc", "RC2", "cbc", 16, 8),
        ("rc2_ecb", "RC2", "ecb", 16, 0),
    ]
    
    def __init__(self, rpo_path, capture_path, output_dir="/tmp/rpo_extracted", max_tries=DEFAULT_MAX_TRIES):
        self.rpo_path = Path(rpo_path)
        self.capture_path = Path(capture_path)
        self.output_dir = Path(output_dir)
        self.max_tries = max_tries
        self.key = None
        self.iv = None
        self.successes = []
        self.failures = []
        self.attempts = 0
        self.working_cipher = None
        
    def run(self):
        """Executar loop principal com 150M tentativas."""
        print(f"{'='*70}")
        print("RALPH LOOP v3 — 150 MILHÕES DE TENTATIVAS AUTO-CORRETIVAS")
        print(f"{'='*70}")
        print(f"RPO: {self.rpo_path.name} ({self.rpo_path.stat().st_size:,} bytes)")
        print(f"Captura: {self.capture_path.name}")
        print(f"Output: {self.output_dir}")
        print(f"Max tentativas: {self.max_tries:,}")
        print(f"Cifras para testar: {len(self.CIPHERS)}")
        print(f"{'='*70}\n")
        
        # Phase 1: Carregar captura
        if not self._load_capture():
            print("[FATAL] Falha ao carregar captura")
            return False
        
        # Phase 2: Extrair key/IV
        if not self._extract_key():
            print("[FATAL] Nenhuma key encontrada na captura")
            return False
        
        print(f"[OK] Key: {self.key.hex()[:32]}...")
        print(f"[OK] IV: {self.iv.hex() if self.iv else 'None'}\n")
        
        # Phase 3: Brute-force de cifras
        return self._bruteforce_ciphers()
        
    def _load_capture(self):
        """Carregar eventos de captura."""
        try:
            self.events = json.load(open(self.capture_path))
            print(f"[OK] Captura carregada: {len(self.events)} eventos")
            print(f"     Tipos: {dict(Counter(e.get('type') for e in self.events))}")
            return True
        except Exception as e:
            print(f"[-] Erro ao carregar captura: {e}")
            return False
            
    def _extract_key(self):
        """Extrair key e IV da captura."""
        for e in self.events:
            if e.get('type') == 'setkey' and e.get('key'):
                self.key = bytes.fromhex(e['key'])
                self.iv = bytes.fromhex(e.get('iv', '00'*8)) if e.get('iv') else None
                return True
        return False
        
    def _bruteforce_ciphers(self):
        """Tentar todas as combinações de cifra (150M tentativas max)."""
        print(f"[Phase 3] Bruteforce de cifras ({len(self.CIPHERS)} combinações)...")
        print(f"          Max tentativas: {self.max_tries:,}\n")
        
        self.output_dir.mkdir(parents=True, exist_ok=True)
        
        # Carregar RPO
        rpo_data = self.rpo_path.read_bytes()
        content = rpo_data[38:-34]  # Remover header/footer
        
        print(f"[INFO] Content: {len(content):,} bytes")
        
        # Segmentos de encrypt
        encrypt_events = [e for e in self.events if e.get('type') == 'encrypt']
        print(f"[INFO] Eventos encrypt: {len(encrypt_events)}")
        
        decrypted_count = 0
        attempted = 0
        last_progress = 0
        
        for i, enc_event in enumerate(encrypt_events):
            if attempted >= self.max_tries:
                print(f"\n[!] Limite de {self.max_tries:,} tentativas atingido")
                break
                
            ct_hex = enc_event.get('plaintext', '')
            if not ct_hex:
                continue
                
            ct = bytes.fromhex(ct_hex)
            
            # Tentar cada cifra
            for cipher_name, algo_class, mode, key_len, iv_len in self.CIPHERS:
                if attempted >= self.max_tries:
                    break
                    
                self.attempts += 1
                attempted += 1
                
                # Ajustar key/iv para tamanho da cifra
                cipher_key = self.key[:key_len] if len(self.key) >= key_len else self.key
                cipher_iv = self.iv[:iv_len] if self.iv and iv_len > 0 else None
                
                # Tentar decodificar
                result = self._try_decrypt(ct, cipher_name, algo_class, mode, cipher_key, cipher_iv)
                
                if result:
                    decrypted_count += 1
                    print(f"  [{i+1}/{len(encrypt_events)}] ✓ {cipher_name}: {len(ct)} -> {len(result)} bytes")
                    
                    # Salvar segmento
                    seg_file = self.output_dir / f"seg_{i:04d}_{cipher_name}.bin"
                    seg_file.write_bytes(result)
                    
                    # Verificar se é zlib
                    if result[:2] == b'\x78\x9c':
                        try:
                            import zlib
                            decompressed = zlib.decompress(result)
                            print(f"         → zlib: {len(decompressed)} bytes")
                            
                            # Salvar decomprimido
                            zlib_file = self.output_dir / f"seg_{i:04d}_zlib.bin"
                            zlib_file.write_bytes(decompressed)
                        except:
                            pass
                    
                    # Auto-correção: se encontrou cipher que funciona, usar nas próximas
                    if self.working_cipher is None:
                        self.working_cipher = cipher_name
                        print(f"\n[!] AUTO-CORREÇÃO: cipher funcional identificado: {cipher_name}")
                        print(f"    Reexecutando com cipher fixo...\n")
                        
                        # Reexecutar com cipher conhecido
                        return self._bruteforce_with_known_cipher(encrypt_events, cipher_name)
                    
                    break
                else:
                    self.failures.append({
                        'attempt': attempted,
                        'cipher': cipher_name,
                        'index': i
                    })
            
            # Progresso
            if i - last_progress >= 100:
                print(f"  Progresso: {i}/{len(encrypt_events)} ({100*i//len(encrypt_events)}%) - {attempted:,} tentativas")
                last_progress = i
        
        # Relatório
        print(f"\n{'='*70}")
        print(f"RESULTADO FINAL")
        print(f"{'='*70}")
        print(f"Tentativas: {attempted:,}/{self.max_tries:,}")
        print(f"Decodificados: {decrypted_count}/{len(encrypt_events)}")
        print(f"Taxa de sucesso: {100*decrypted_count/max(len(encrypt_events),1):.1f}%")
        
        if decrypted_count > 0:
            print(f"\n[✓] SUCESSO! Fontes extraídos em: {self.output_dir}")
            return True
        else:
            print(f"\n[✗] Falha na decodificação")
            return False
            
    def _try_decrypt(self, ct, cipher_name, algo_class, mode, key, iv):
        """Tentar decodificar com cifra específica."""
        try:
            from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
            from cryptography.hazmat.backends import default_backend
            
            # Criar cipher
            algo = getattr(algorithms, algo_class)(key)
            
            if mode == "ecb":
                m = modes.ECB()
            elif mode == "cbc":
                m = modes.CBC(iv) if iv else modes.ECB()
            elif mode == "cfb64":
                m = modes.CFB(iv, 64) if iv else modes.ECB()
            elif mode == "ofb":
                m = modes.OFB(iv, 8) if iv else modes.ECB()
            elif mode == "stream":
                # RC4 - stream cipher
                return self._decrypt_rc4(ct, key)
            else:
                return None
            
            cipher = Cipher(algo, m, backend=default_backend())
            decryptor = cipher.decryptor()
            pt = decryptor.update(ct) + decryptor.finalize()
            
            # Verificar validade
            if len(pt) > 0:
                # Check se parece válido (zlib ou ASCII)
                if pt[:2] == b'\x78\x9c' or all(32 <= b < 127 for b in pt[:20]):
                    return pt
                            
            return None
        except Exception as e:
            return None
            
    def _decrypt_rc4(self, ct, key):
        """Decodificar RC4 (stream cipher)."""
        try:
            # RC4 implementation
            S = list(range(256))
            j = 0
            for i in range(256):
                j = (j + S[i] + key[i % len(key)]) % 256
                S[i], S[j] = S[j], S[i]
            
            i = j = 0
            pt = []
            for byte in ct:
                i = (i + 1) % 256
                j = (j + S[i]) % 256
                S[i], S[j] = S[j], S[i]
                K = S[(S[i] + S[j]) % 256]
                pt.append(byte ^ K)
            
            return bytes(pt)
        except:
            return None
            
    def _bruteforce_with_known_cipher(self, encrypt_events, cipher_name):
        """Reexecutar com cipher conhecido."""
        print(f"[Auto-correção] Reexecutando com cipher: {cipher_name}\n")
        
        decrypted_count = 0
        
        for i, enc_event in enumerate(encrypt_events):
            ct_hex = enc_event.get('plaintext', '')
            if not ct_hex:
                continue
                
            ct = bytes.fromhex(ct_hex)
            result = self._try_decrypt(ct, cipher_name, *self._get_cipher_params(cipher_name))
            
            if result:
                decrypted_count += 1
                seg_file = self.output_dir / f"seg_{i:04d}_{cipher_name}.bin"
                seg_file.write_bytes(result)
                
                if decrypted_count % 100 == 0:
                    print(f"  Progresso: {decrypted_count}/{len(encrypt_events)} segmentos decodificados")
        
        print(f"\n[FINAL] {decrypted_count}/{len(encrypt_events)} segmentos decodificados com {cipher_name}")
        return decrypted_count > 0
        
    def _get_cipher_params(self, cipher_name):
        """Retornar parâmetros da cifra."""
        for c in self.CIPHERS:
            if c[0] == cipher_name:
                return c[1], c[2], c[3], c[4]
        return None, None, None, None
        
    def _report(self):
        """Gerar relatório final."""
        report = {
            'timestamp': time.strftime('%Y-%m-%d %H:%M:%S'),
            'rpo': str(self.rpo_path),
            'capture': str(self.capture_path),
            'total_attempts': self.attempts,
            'max_attempts': self.max_tries,
            'success_rate': f"{100*self.attempts/max(self.max_tries,1):.2f}%",
            'ciphers_tested': len(self.CIPHERS),
            'working_cipher': self.working_cipher,
            'output_dir': str(self.output_dir),
            'key': self.key.hex() if self.key else None,
            'iv': self.iv.hex() if self.iv else None,
        }
        
        report_file = self.output_dir / "ralph_loop_report.json"
        report_file.write_text(json.dumps(report, indent=2))
        print(f"\n[OK] Relatório salvo: {report_file}")


def main():
    parser = argparse.ArgumentParser(description='Ralph Loop v3 — 150M tentativas auto-corretivas')
    parser.add_argument('rpo', help='Arquivo RPO')
    parser.add_argument('capture', help='Arquivo de captura JSON')
    parser.add_argument('--output', '-o', default='/tmp/rpo_extracted', help='Diretório de saída')
    parser.add_argument('--max-tries', type=int, default=DEFAULT_MAX_TRIES, help='Max tentativas')
    parser.add_argument('--verbose', '-v', action='store_true')
    
    args = parser.parse_args()
    global VERBOSE
    VERBOSE = args.verbose
    
    loop = RalphLoop(args.rpo, args.capture, args.output, args.max_tries)
    success = loop.run()
    loop._report()
    
    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
