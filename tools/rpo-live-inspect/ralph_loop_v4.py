#!/usr/bin/env python3
"""
Ralph Loop v4 — Corrigido: re-criptografa plaintext e busca no RPO.

FLUXO:
  1. Captura online via LD_PRELOAD (key + IV + plaintext)
  2. Para cada plaintext capturado:
     a. Tentar criptografar com cada cifra conhecida
     b. Buscar ciphertext resultante no RPO
     c. Se encontrado → segmento decodificado!
  3. Auto-correção: se cipher X funcionar, usar nos próximos
"""

import json
import sys
import time
from pathlib import Path
from collections import Counter

try:
    from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
    from cryptography.hazmat.backends import default_backend
    HAS_CRYPTO = True
except ImportError:
    HAS_CRYPTO = False
    print("Erro: Instale cryptography: pip install cryptography")
    sys.exit(1)


class RalphLoopV4:
    """Versão corrigida que re-criptografa e busca no RPO."""
    
    CIPHERS = [
        ("des_ede_cbc", "TripleDES", "cbc", 24, 8),
        ("des_ede_ecb", "TripleDES", "ecb", 24, 0),
        ("cast5_cbc", "CAST5", "cbc", 16, 8),
        ("cast5_ecb", "CAST5", "ecb", 16, 0),
        ("bf_cbc", "Blowfish", "cbc", 16, 8),
        ("bf_ecb", "Blowfish", "ecb", 16, 0),
        ("rc4", "RC4", "stream", 16, 0),
    ]
    
    def __init__(self, rpo_path, capture_path, output_dir="/tmp/rpo_extracted"):
        self.rpo_path = Path(rpo_path)
        self.capture_path = Path(capture_path)
        self.output_dir = Path(output_dir)
        self.key = None
        self.iv = None
        self.working_cipher = None
        
    def run(self):
        print(f"{'='*70}")
        print("RALPH LOOP v4 — Re-criptografia + Busca no RPO")
        print(f"{'='*70}")
        
        # Carregar captura
        events = json.load(open(self.capture_path))
        print(f"[OK] Captura: {len(events)} eventos")
        
        # Extrair key
        for e in events:
            if e.get('type') == 'setkey' and e.get('key'):
                self.key = bytes.fromhex(e['key'])
                self.iv = bytes.fromhex(e.get('iv', '00'*8)) if e.get('iv') else None
                print(f"[OK] Key: {e['key'][:32]}...")
                print(f"[OK] IV: {e.get('iv', 'None')}")
                break
        
        if not self.key:
            print("[FATAL] Nenhuma key encontrada")
            return False
        
        # Carregar RPO
        rpo_data = self.rpo_path.read_bytes()
        content = rpo_data[38:-34]  # Remover header/footer
        print(f"[OK] RPO: {len(rpo_data):,} bytes, content: {len(content):,} bytes")
        
        # Eventos de encrypt (plaintext antes de cifrar)
        encrypt_events = [e for e in events if e.get('type') == 'encrypt']
        print(f"[INFO] Plaintexts para criptografar: {len(encrypt_events)}\n")
        
        # Criar output
        self.output_dir.mkdir(parents=True, exist_ok=True)
        
        # Para cada plaintext, tentar criptografar e buscar no RPO
        matched = 0
        for i, evt in enumerate(encrypt_events):
            pt_hex = evt.get('plaintext', '')
            if not pt_hex:
                continue
            
            pt = bytes.fromhex(pt_hex)
            
            # Tentar cada cifra
            for cipher_name, algo_class, mode, key_len, iv_len in self.CIPHERS:
                # Ajustar key/iv
                k = self.key[:key_len] if len(self.key) >= key_len else self.key
                iv = self.iv[:iv_len] if self.iv and iv_len > 0 else None
                
                # Criptografar
                ct = self._encrypt(pt, cipher_name, algo_class, mode, k, iv)
                if ct is None:
                    continue
                
                # Buscar ciphertext no RPO
                pos = content.find(ct)
                if pos >= 0:
                    matched += 1
                    print(f"  [{i+1}/{len(encrypt_events)}] ✓ {cipher_name}: pt={len(pt)} -> ct={len(ct)} @ offset {pos}")
                    
                    # Salvar segmento
                    seg_file = self.output_dir / f"seg_{i:04d}_{cipher_name}.bin"
                    seg_file.write_bytes(ct)
                    
                    # Auto-correção: usar cipher encontrado
                    if self.working_cipher is None:
                        self.working_cipher = cipher_name
                        print(f"\n[!] AUTO-CORREÇÃO: cipher funcional = {cipher_name}")
                        print(f"    Reexecutando com cipher fixo...\n")
                        
                        # Reexecutar todo com cipher fixo
                        return self._reencrypt_with_fixed_cipher(encrypt_events, cipher_name, content)
                    
                    break
        
        print(f"\n{'='*70}")
        print(f"RESULTADO: {matched}/{len(encrypt_events)} segmentos localizados")
        print(f"{'='*70}")
        return matched > 0
        
    def _encrypt(self, pt, cipher_name, algo_class, mode, key, iv):
        """Criptografar plaintext com cifra específica."""
        try:
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
                return self._encrypt_rc4(pt, key)
            else:
                return None
            
            cipher = Cipher(algo, m, backend=default_backend())
            encryptor = cipher.encryptor()
            return encryptor.update(pt) + encryptor.finalize()
        except Exception as e:
            return None
            
    def _encrypt_rc4(self, pt, key):
        """Criptografar com RC4."""
        try:
            S = list(range(256))
            j = 0
            for i in range(256):
                j = (j + S[i] + key[i % len(key)]) % 256
                S[i], S[j] = S[j], S[i]
            
            i = j = 0
            ct = []
            for byte in pt:
                i = (i + 1) % 256
                j = (j + S[i]) % 256
                S[i], S[j] = S[j], S[i]
                K = S[(S[i] + S[j]) % 256]
                ct.append(byte ^ K)
            
            return bytes(ct)
        except:
            return None
            
    def _reencrypt_with_fixed_cipher(self, encrypt_events, cipher_name, content):
        """Reexecutar com cipher fixo."""
        print(f"[Reexecutando] Usando cipher fixo: {cipher_name}\n")
        
        matched = 0
        for i, evt in enumerate(encrypt_events):
            pt_hex = evt.get('plaintext', '')
            if not pt_hex:
                continue
            
            pt = bytes.fromhex(pt_hex)
            ct = self._encrypt(pt, cipher_name, *self._get_params(cipher_name))
            
            if ct and ct in content:
                matched += 1
                pos = content.find(ct)
                
                seg_file = self.output_dir / f"seg_{i:04d}_{cipher_name}.bin"
                seg_file.write_bytes(ct)
                
                if matched % 50 == 0:
                    print(f"  Progresso: {matched}/{len(encrypt_events)}")
        
        print(f"\n{'='*70}")
        print(f"FINAL: {matched}/{len(encrypt_events)} segmentos localizados")
        print(f"{'='*70}")
        return matched > 0
        
    def _get_params(self, cipher_name):
        """Retornar parâmetros da cifra."""
        for c in self.CIPHERS:
            if c[0] == cipher_name:
                return c[1], c[2], c[3], c[4]
        return None, None, None, None


def main():
    if len(sys.argv) < 3:
        print("Uso: python3 ralph_loop_v4.py <rpo> <capture.json> [output]")
        sys.exit(1)
    
    rpo = sys.argv[1]
    capture = sys.argv[2]
    output = sys.argv[3] if len(sys.argv) > 3 else "/tmp/rpo_extracted"
    
    loop = RalphLoopV4(rpo, capture, output)
    success = loop.run()
    
    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
