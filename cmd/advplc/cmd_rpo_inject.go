package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/rpo"
	"github.com/advpl/compiler/pkg/rpo/injector"
)

func cmdRpoInject(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf(`uso: advplc rpo inject <arquivo.rpo> <captura.json> [opcões]

Opções:
  --list                Lista registros APO encontrados
  --inject <nome>=<arquivo.bytecode>  Substitui bytecode de uma função
  --out <arquivo>       Arquivo de saída (padrão: substitui o original)
  --verbose             Mostra detalhes da decodificação

Exemplo:
  advplc rpo inject custom.rpo captura.json --list
  advplc rpo inject custom.rpo captura.json --inject MinhaFuncao=saída.bytecode -o novo.rpo`)
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
	fmt.Printf("  Magic: %s\n", f.FooterMagic)
	fmt.Printf("\n")

	inj, err := injector.NewInjector(rpoData)
	if err != nil {
		return fmt.Errorf("criando injector: %w", err)
	}

	err = inj.LoadCapture(capturePath)
	if err != nil {
		return fmt.Errorf("carregando captura: %w", err)
	}

	body, decoded := inj.DecodeBody()
	
	injectArgs := map[string]string{}
	outPath := ""

	for i := 2; i < len(args); i++ {
		switch args[i] {
			// verbose = true
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
			// já tratado
		}
	}

	fmt.Printf("\nDecodificados: %d segmentos\n", len(decoded))

	records := injector.ExtractAPORecords(body)
	injector.PrintAPORecords(records)

	if len(injectArgs) > 0 {
		fmt.Printf("\n=== Processando injeções ===\n")
		
		for funcName, path := range injectArgs {
			fmt.Printf("\nInjetando: %s <- %s\n", funcName, path)
			
			binData, err := compiler.SerializeBytecode(path)
			if err != nil {
				fmt.Printf("  ERRO: convertendo bytecode: %v\n", err)
				continue
			}
			
			jsonSize := fileSize(path)
			fmt.Printf("  Bytecode: %d bytes (JSON) -> %d bytes (binário)\n", jsonSize, len(binData))
			
			// Encontrar registro pelo nome
			idx := -1
			for i, rec := range records {
				if strings.EqualFold(rec.Name, funcName) {
					idx = i
					break
				}
			}
			
			if idx < 0 {
				fmt.Printf("  AVISO: função '%s' não encontrada nos registros APO\n", funcName)
				continue
			}
			
			err = inj.ReplaceAPO(idx, binData)
			if err != nil {
				fmt.Printf("  ERRO: %v\n", err)
				continue
			}
			
			fmt.Printf("  ✓ Injetado com sucesso\n")
		}
		
		if outPath == "" {
			outPath = strings.TrimSuffix(rpoPath, ".rpo") + "_injected.rpo"
		}
		
		fmt.Printf("\nSalvando em: %s\n", outPath)
		err = inj.Save(outPath)
		if err != nil {
			return fmt.Errorf("salvando RPO: %w", err)
		}
		
		fmt.Printf("\n✅ Conclusão\n")
		fmt.Printf("   Nota: Recriptografia não implementada ainda.\n")
		fmt.Printf("   O RPO modificado requer novas chaves de criptografia.\n")
	}

	return nil
}

func fileSize(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(info.Size())
}
