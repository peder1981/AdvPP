package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/advpl/compiler/pkg/rpo"
)

// apoExtsDefine quais arquivos de um diretório são blobs APO.
var apoExts = map[string]bool{
	".PRW": true, ".TLPP": true, ".PRX": true,
	".APH": true, ".APW": true, ".PRG": true,
}

type apoJSON struct {
	FileName       string              `json:"file"`
	Kind           string              `json:"kind"`
	Size           int                 `json:"size"`
	Identifiers    []string            `json:"identifiers"`
	Literals       []string            `json:"literals"`
	Snippets       []string            `json:"snippets"`
	CallCandidates []rpo.CallCandidate `json:"call_candidates"`
}

// cmdRpoApo implementa "advplc rpo apo <arquivo|dir> [--catalog f]
// [--out dir] [--format json|md]".
func cmdRpoApo(args []string) error {
	fs := flag.NewFlagSet("apo", flag.ContinueOnError)
	catalogPath := fs.String("catalog", "", "arquivo de catálogo de nomes (oráculo call-graph)")
	outDir := fs.String("out", "", "diretório de saída (padrão: stdout)")
	format := fs.String("format", "json", "formato: json|md")
	// Aceita flags antes OU depois do caminho (o flag padrão para no
	// primeiro argumento não-flag — sem isto, "rpo apo <dir> --out x"
	// engoliria os flags como posicionais).
	flagArgs, positional := apoSplitArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if len(positional) == 0 {
		return fmt.Errorf("uso: advplc rpo apo [--catalog f] [--out dir] [--format json|md] <arquivo|dir>")
	}

	var catalog map[string]bool
	if *catalogPath != "" {
		var err error
		catalog, err = rpo.LoadApoCatalog(*catalogPath)
		if err != nil {
			return fmt.Errorf("carregando catálogo: %w", err)
		}
	}

	files, err := apoCollectFiles(positional[0])
	if err != nil {
		return err
	}

	reports := make([]apoJSON, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("lendo %s: %w", path, err)
		}
		blob, err := rpo.ParseApoBlob(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AVISO: %s: %v\n", filepath.Base(path), err)
			continue
		}
		blob.ExtractStrings()
		blob.Classify(catalog)
		reports = append(reports, apoJSON{
			FileName:       blob.FileName,
			Kind:           blob.Kind.String(),
			Size:           blob.Size,
			Identifiers:    blob.Identifiers,
			Literals:       blob.Literals,
			Snippets:       blob.Snippets,
			CallCandidates: blob.CallCandidates,
		})
	}

	var payload []byte
	var filename string
	switch *format {
	case "json":
		payload, err = json.MarshalIndent(reports, "", "  ")
		filename = "apo_report.json"
	case "md":
		payload = apoRenderMD(reports, *catalogPath)
		filename = "apo_report.md"
	default:
		return fmt.Errorf("formato desconhecido %q (json|md)", *format)
	}
	if err != nil {
		return err
	}

	if *outDir == "" {
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(*outDir, filename)
	if err := os.WriteFile(dest, payload, 0644); err != nil {
		return err
	}
	fmt.Printf("Relatório: %s (%d blobs)\n", dest, len(reports))
	return nil
}

// apoSplitArgs separa flags de posicionais. Regra: todo "-x"/"--x" sem "="
// que é flag conhecido de valor consome o próximo argumento.
func apoSplitArgs(args []string) (flagArgs, positional []string) {
	takesValue := map[string]bool{
		"-catalog": true, "--catalog": true,
		"-out": true, "--out": true,
		"-format": true, "--format": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			if !strings.Contains(a, "=") && takesValue[a] && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return flagArgs, positional
}

// apoCollectFiles aceita arquivo único ou diretório (walk 1 nível).
func apoCollectFiles(arg string) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	entries, err := os.ReadDir(arg)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if apoExts[strings.ToUpper(filepath.Ext(e.Name()))] {
			files = append(files, filepath.Join(arg, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("nenhum blob APO em %s", arg)
	}
	return files, nil
}

// apoRenderMD gera o relatório markdown (tabela resumo + seções por arquivo).
func apoRenderMD(reports []apoJSON, catalogName string) []byte {
	var b strings.Builder
	b.WriteString("# Relatório de desmontagem APO\n\n")
	fmt.Fprintf(&b, "Catálogo: `%s` · Blobs: %d\n\n", catalogName, len(reports))
	b.WriteString("| Arquivo | Kind | Bytes | Identificadores | Literais | Snippets | Candidatos |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|\n")
	for _, r := range reports {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d |\n",
			r.FileName, r.Kind, r.Size,
			len(r.Identifiers), len(r.Literals), len(r.Snippets), len(r.CallCandidates))
	}
	return []byte(b.String())
}
