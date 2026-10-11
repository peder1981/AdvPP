package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/advpl/compiler/pkg/rpo"
	"github.com/advpl/compiler/pkg/rpo/injector"
)

func cmdRpoInject(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf(`uso: advplc rpo inject <arquivo.rpo> <captura.json> [opções]

Opções:
  --list                Lista registros APO encontrados
  --list-admin          Lista entries do índice admin (se parseável)
  --inject <nome>=<arq> Substitui bytecode de uma função (repetível)
  --out <arquivo>       Arquivo de saída
  --verbose             Mostra detalhes

Exemplo:
  advplc rpo inject custom.rpo captura.json --list
  advplc rpo inject custom.rpo captura.json --inject Fun1=bc1.bin --inject Fun2=bc2.bin --out out.rpo`)
	}

	rpoPath := args[0]
	capturePath := args[1]

	rpoData, err := os.ReadFile(rpoPath)
	if err != nil {
		return fmt.Errorf("lendo %s: %w", rpoPath, err)
	}

	f, err := rpo.Parse(rpoData)
	if err != nil {
		return fmt.Errorf("parse do RPO: %w", err)
	}

	fmt.Printf("RPO: %s\n", rpoPath)
	fmt.Printf("  Nome: %s\n", f.Name)
	fmt.Printf("  Tamanho: %d bytes\n", len(rpoData))
	fmt.Printf("  Admin: %d bytes\n", len(f.AdminSection))
	fmt.Printf("  Body: %d bytes\n", len(f.Body))
	fmt.Printf("  Magic: %s\n\n", f.FooterMagic)

	inj, err := injector.NewInjector(rpoData)
	if err != nil {
		return fmt.Errorf("criando injector: %w", err)
	}

	err = inj.LoadCapture(capturePath)
	if err != nil {
		return fmt.Errorf("carregando captura: %w", err)
	}

	_, decoded := inj.DecodeBody()
	inj.DetectSimpleMode()

	injectArgs := map[string]string{}
	outPath := ""
	listOnly := false
	listAdmin := false

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--out", "-o":
			if i+1 < len(args) {
				outPath = args[i+1]
				i++
			}
		case "--inject":
			if i+1 < len(args) {
				parts := strings.SplitN(args[i+1], "=", 2)
				if len(parts) == 2 {
					injectArgs[parts[0]] = parts[1]
				}
				i++
			}
		case "--list":
			listOnly = true
		case "--list-admin":
			listAdmin = true
		}
	}

	fmt.Printf("\nDecodificados: %d segmentos\n", len(decoded))

	// List APO records
	records := inj.GetAPORecords()
	injector.PrintAPORecords(records)

	// List admin index if requested
	if listAdmin {
		if err := inj.ParseAdminIndex(); err == nil {
			injector.PrintAdminIndex(inj.AdminIndex())
		}
	}

	if listOnly || listAdmin {
		if outPath == "" {
			outPath = strings.TrimSuffix(rpoPath, ".rpo") + "_analyzed.rpo"
		}
		fmt.Printf("\nSalvando RPO em: %s\n", outPath)
		return inj.SavePadded(outPath)
	}

	if len(injectArgs) == 0 {
		fmt.Println("\nNenhuma injeção especificada. Use --inject Nome=arquivo.bytecode")
		return nil
	}

	// Batch injection
	codeMap := make(map[string][]byte, len(injectArgs))
	for k, v := range injectArgs {
		b, err := os.ReadFile(v)
		if err != nil {
			return fmt.Errorf("lendo %s: %w", v, err)
		}
		codeMap[k] = b
	}
	err = inj.ReplaceFunctions(codeMap)
	if err != nil {
		return fmt.Errorf("injeção falhou: %w", err)
	}

	if outPath == "" {
		outPath = strings.TrimSuffix(rpoPath, ".rpo") + "_injected.rpo"
	}

	fmt.Printf("\nSalvando em: %s\n", outPath)
	err = inj.SavePadded(outPath)
	if err != nil {
		return fmt.Errorf("salvando RPO: %w", err)
	}

	fmt.Printf("\n✅ Conclusão\n")
	fmt.Printf("   RPO salvo com %d função(ões) injetada(s).\n", len(injectArgs))
	return nil
}
