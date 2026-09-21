package main

// cmd_rpo_regions.go — `advplc rpo regions` classifica o conteúdo do RPO
// em janelas (zero / ciphertext / mixed / plaintext) por entropia de
// Shannon, sem inventar estrutura. É a ferramenta HONESTA que substitui
// as tentativas antigas de "identificar rotinas" por regex sobre cifra.
// Ver pkg/rpo/forensics.go e docs/RPO-GROUND-TRUTH.md.

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/advpl/compiler/pkg/rpo"
)

func cmdRpoRegions(args []string) error {
	fs := flag.NewFlagSet("regions", flag.ContinueOnError)
	window := fs.Int("window", 4096, "tamanho da janela em bytes")
	top := fs.Int("top", 0, "mostrar as N janelas de maior entropia")
	stringsOnly := fs.Bool("strings", false, "extrair strings somente de janelas plaintext")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("uso: advplc rpo regions [--window N] [--top N] [--strings] <arquivo.rpo>")
	}
	path := fs.Arg(0)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("lendo %s: %w", path, err)
	}
	f, err := rpo.Parse(data)
	if err != nil {
		return fmt.Errorf("parse do RPO: %w", err)
	}
	content := append(append([]byte{}, f.AdminSection...), f.Body...)

	regions := rpo.ClassifyRegions(content, *window)
	sum := rpo.SummarizeRegions(regions)

	fmt.Printf("RPO: %s\n", path)
	fmt.Printf("Conteúdo: %d bytes (admin+body), janela=%d\n\n", len(content), *window)
	fmt.Printf("Entropia média: %.4f bits/byte\n", sum.MeanEntropy)
	fmt.Printf("  zero       : %12d bytes (%.2f%%)\n", sum.ZeroBytes, pct(sum.ZeroBytes, sum.TotalBytes))
	fmt.Printf("  ciphertext : %12d bytes (%.2f%%)\n", sum.CiphertextBytes, pct(sum.CiphertextBytes, sum.TotalBytes))
	fmt.Printf("  mixed      : %12d bytes (%.2f%%)\n", sum.MixedBytes, pct(sum.MixedBytes, sum.TotalBytes))
	fmt.Printf("  plaintext  : %12d bytes (%.2f%%)\n", sum.PlaintextBytes, pct(sum.PlaintextBytes, sum.TotalBytes))
	fmt.Println()

	if sum.PlaintextBytes == 0 {
		fmt.Println("VEREDITO: nenhuma região plaintext — o conteúdo é cifra/comprimido.")
		fmt.Println("Não há estrutura legível sem uma captura de chave ao vivo")
		fmt.Println("(`advplc rpo decrypt <rpo> <captura.json>`).")
		fmt.Println("Qualquer 'nome de rotina' obtido por regex aqui é falso positivo.")
		fmt.Println()
	}

	if *top > 0 {
		sorted := append([]rpo.Region{}, regions...)
		rpo.SortRegionsByEntropy(sorted)
		fmt.Printf("Top %d janelas por entropia:\n", *top)
		for i, r := range sorted[:minInt(*top, len(sorted))] {
			fmt.Printf("  [%d] +%d size=%d ent=%.3f uniq=%d zero=%.1f%% %s\n",
				i+1, r.Offset, r.Size, r.Entropy, r.UniqueByte, r.ZeroRatio*100, r.Kind)
		}
		fmt.Println()
	}

	if *stringsOnly {
		matches := rpo.ExtractStringsFromPlaintext(content, *window, 4)
		fmt.Printf("Strings em regiões plaintext: %d\n", len(matches))
		max := len(matches)
		if max > 100 {
			max = 100
		}
		for _, m := range matches[:max] {
			fmt.Printf("  +%-10d %s\n", m.Offset, m.Text)
		}
		if len(matches) > max {
			fmt.Printf("  ... e %d mais\n", len(matches)-max)
		}
	}

	return nil
}

func pct(a, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(a) / float64(total)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// evita import não usado de sort em builds futuras
var _ = sort.Ints
