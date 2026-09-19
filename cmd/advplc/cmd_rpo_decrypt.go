package main

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/advpl/compiler/pkg/rpo"
)

// cmdRpoDecrypt implementa "advplc rpo decrypt <arquivo.rpo> <captura.json>".
//
// [!] Isto NÃO é descriptografia offline mágica — o RPO usa uma tabela
// rotativa de ~12 cifras legadas do OpenSSL escolhidas por chamada
// dentro de tCryptoEVP::Encrypt, com chave/IV efêmeros por sessão de
// compilação (ver docs/rpo-format-sonnet.md). Não há como decodificar
// um RPO arbitrário só com o arquivo em disco. O que ESTE comando faz é
// usar uma CAPTURA AO VIVO prévia (gerada durante a MESMA compilação que
// produziu o RPO, via tools/rpo-live-inspect/extract_rpo.py com gdb, ou
// via tools/rpo-live-inspect/rpo_key_hook com LD_PRELOAD — os dois
// produzem o mesmo formato JSON) para reproduzir e verificar o
// conteúdo real, com a cifra correta identificada por segmento.
func cmdRpoDecrypt(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf(`uso: advplc rpo decrypt <arquivo.rpo> <captura.json>

<captura.json> é o arquivo gerado DURANTE A MESMA COMPILAÇÃO que
produziu <arquivo.rpo> — via 'advplc rpo extract <rpo> --auto' (gdb) ou
via LD_PRELOAD=tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook.so
(ver docs/rpo-format-sonnet.md). Sem essa captura não há como
decodificar o RPO — a chave/cifra só existem em memória durante a
compilação, não são recuperáveis do arquivo sozinho.`)
	}
	rpoPath, capturePath := args[0], args[1]

	rpoData, err := os.ReadFile(rpoPath)
	if err != nil {
		return fmt.Errorf("lendo %s: %w", rpoPath, err)
	}
	f, err := rpo.Parse(rpoData)
	if err != nil {
		return fmt.Errorf("parse do RPO: %w", err)
	}

	events, err := rpo.LoadCaptureEvents(capturePath)
	if err != nil {
		return fmt.Errorf("lendo captura %s: %w", capturePath, err)
	}
	segments := rpo.MergeCaptureSegments(events)

	fmt.Printf("RPO: %s (admin=%dB body=%dB)\n", rpoPath, len(f.AdminSection), len(f.Body))
	fmt.Printf("Captura: %s (%d segmentos)\n\n", capturePath, len(segments))

	decoded := 0
	for _, seg := range segments {
		if seg.Cipher == "" || seg.Key == "" || seg.Plaintext == "" {
			continue
		}
		key, err := hex.DecodeString(seg.Key)
		if err != nil {
			fmt.Printf("#%d %s: chave hex inválida, pulando\n", seg.N, seg.Cipher)
			continue
		}
		var iv []byte
		if seg.IV != "" {
			iv, _ = hex.DecodeString(seg.IV)
		}
		plain, err := hex.DecodeString(seg.Plaintext)
		if err != nil {
			fmt.Printf("#%d %s: plaintext hex inválido, pulando\n", seg.N, seg.Cipher)
			continue
		}

		ct, err := rpo.EncryptSegment(seg.Cipher, key, iv, plain)
		if err != nil {
			fmt.Printf("#%d %s: %v\n", seg.N, seg.Cipher, err)
			continue
		}

		where := "SEM MATCH"
		if idx := bytes.Index(f.Body, ct); idx >= 0 {
			where = fmt.Sprintf("body.bin offset %d", idx)
		} else if idx := bytes.Index(f.AdminSection, ct); idx >= 0 {
			where = fmt.Sprintf("admin_section offset %d", idx)
		}

		fmt.Printf("#%d %s (%d bytes): %s\n", seg.N, seg.Cipher, len(plain), where)

		if len(plain) >= 2 && plain[0] == 0x78 && (plain[1] == 0x9c || plain[1] == 0x01 || plain[1] == 0xda) {
			if r, err := zlib.NewReader(bytes.NewReader(plain)); err == nil {
				inflated, err := io.ReadAll(r)
				r.Close()
				if err == nil {
					preview := inflated
					if len(preview) > 200 {
						preview = preview[:200]
					}
					fmt.Printf("    zlib inflate OK (%d bytes): %q\n", len(inflated), preview)
				}
			}
		}
		if where != "SEM MATCH" {
			decoded++
		}
	}

	fmt.Printf("\n%d/%d segmentos decodificados e confirmados contra o RPO real.\n", decoded, len(segments))
	if decoded == 0 {
		return fmt.Errorf("nenhum segmento decodificado — captura não corresponde a este RPO, ou cifras não suportadas (ver pkg/rpo/idea.go para limitações conhecidas)")
	}
	return nil
}
