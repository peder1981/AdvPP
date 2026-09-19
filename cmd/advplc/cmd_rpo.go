package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/advpl/compiler/pkg/rpo"
)

// cmdRpo implementA "advplc rpo <subcomando>" — inspeção e decomposição de
// arquivos RPO do Protheus. Ver docs/rpo-format.md para o que é confirmado
// e o que continua opaco (a maior parte do conteúdo compilado não é
// decifrada, só o container é lido/regravado com segurança).
func cmdRpo(args []string) error {
	if len(args) == 0 {
		return rpoUsageError()
	}
	switch args[0] {
	case "info":
		if len(args) < 2 {
			return fmt.Errorf("uso: advplc rpo info <arquivo.rpo>")
		}
		return rpoInfo(args[1])
	case "identify":
		if len(args) < 2 {
			return fmt.Errorf("uso: advplc rpo identify <arquivo.rpo>")
		}
		return rpoIdentify(args[1])
	case "decompose":
		if len(args) < 3 {
			return fmt.Errorf("uso: advplc rpo decompose <arquivo.rpo> <diretorio-saida>")
		}
		return rpoDecompose(args[1], args[2])
	case "extract":
		if len(args) < 2 {
			return fmt.Errorf("uso: advplc rpo extract <arquivo.rpo> [--auto]")
		}
		return cmdRpoExtract(args[1:], len(args) > 2 && args[2] == "--auto")
	case "build":
		if len(args) < 3 {
			return fmt.Errorf("uso: advplc rpo build <diretorio-decomposto> <arquivo.rpo>")
		}
		return rpoBuild(args[1], args[2])
	case "decrypt":
		return cmdRpoDecrypt(args[1:])
	default:
		return rpoUsageError()
	}
}

func rpoUsageError() error {
	return fmt.Errorf(`uso: advplc rpo <subcomando>

Subcomandos:
  info <arquivo.rpo>                        mostra metadados do container
  identify <arquivo.rpo>                    identifica o tipo de RPO automaticamente
  decompose <arquivo.rpo> <dir-saida>       decompõe em arquivos (header/admin/body/footer)
  build <dir-decomposto> <arquivo.rpo>      recompõe um RPO a partir de uma decomposição
  extract <arquivo.rpo> [--auto]            extrai lista de funções (requer appserver rodando)
  decrypt <arquivo.rpo> <captura.json>      decodifica segmentos usando uma captura ao vivo prévia

AVISO: o compilador lê/escreve a estrutura de CONTAINER do RPO (cabeçalho,
ponteiro de auto-referência, footer) de forma segura e verificada. O conteúdo
compilado (P-Code) é cifrado com uma tabela rotativa de ~12 cifras legadas do
OpenSSL (DES/3DES/RC4/RC5/CAST5/Blowfish/RC2 — nunca AES fixo, ver
docs/rpo-format-sonnet.md), com chave/IV efêmeros por sessão de compilação —
só decodificável com uma captura ao vivo feita durante a MESMA compilação
('extract --auto' ou o hook em tools/rpo-live-inspect/rpo_key_hook), nunca a
partir do arquivo .rpo sozinho. Ver docs/rpo-format.md.`)
}

func rpoInfo(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := rpo.Parse(data)
	if err != nil {
		return err
	}
	fmt.Printf("Arquivo ........: %s\n", path)
	fmt.Printf("Tamanho ........: %d bytes\n", len(data))
	fmt.Printf("Nome do RPO ....: %s\n", f.Name)
	fmt.Printf("SelfOffset .....: %d (0x%X)\n", f.SelfOffset, f.SelfOffset)
	fmt.Printf("Admin section ..: %d bytes (opaco)\n", len(f.AdminSection))
	fmt.Printf("Body ...........: %d bytes (opaco — P-Code + estruturas internas)\n", len(f.Body))
	fmt.Printf("Footer magic ...: %s\n", f.FooterMagic)
	fmt.Printf("Footer trailer .: %d bytes (opaco), hex=%s\n", len(f.FooterTrail), strings.ToLower(strings.ReplaceAll(fmt.Sprintf("%x", f.FooterTrail), " ", "")))
	return nil
}

func rpoIdentify(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	profile, err := rpo.Identify(data)
	if err != nil {
		return err
	}
	fmt.Print(profile.FormatSummary())
	return nil
}

func rpoDecompose(path, outDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := rpo.Parse(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	header := f.Bytes()[:rpo.HeaderSize]
	files := map[string][]byte{
		"header.bin":        header,
		"admin_section.bin": f.AdminSection,
		"body.bin":          f.Body,
		"footer_magic.txt":  []byte(f.FooterMagic),
		"footer_trail.bin":  f.FooterTrail,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(outDir, name), content, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("Decomposto em %s (nome=%s, selfOffset=%d)\n", outDir, f.Name, f.SelfOffset)
	fmt.Println("AVISO: admin_section.bin e body.bin são blobs opacos (P-Code + estruturas internas não decifradas) — preservados, não interpretados.")
	return nil
}

func rpoBuild(inDir, outPath string) error {
	header, err := os.ReadFile(filepath.Join(inDir, "header.bin"))
	if err != nil {
		return err
	}
	admin, err := os.ReadFile(filepath.Join(inDir, "admin_section.bin"))
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(inDir, "body.bin"))
	if err != nil {
		return err
	}
	magic, err := os.ReadFile(filepath.Join(inDir, "footer_magic.txt"))
	if err != nil {
		return err
	}
	trail, err := os.ReadFile(filepath.Join(inDir, "footer_trail.bin"))
	if err != nil {
		return err
	}
	if len(header) < rpo.HeaderSize {
		return fmt.Errorf("header.bin menor que %d bytes", rpo.HeaderSize)
	}
	name := string(header[rpo.HeaderPointerSize:rpo.HeaderSize])
	for i, b := range name {
		if b == 0 {
			name = name[:i]
			break
		}
	}

	f := &rpo.File{
		Header:       header[:rpo.HeaderSize],
		Name:         name,
		AdminSection: admin,
		Body:         body,
		FooterMagic:  string(magic),
		FooterTrail:  trail,
	}
	if err := os.WriteFile(outPath, f.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("RPO reconstruído em %s (%d bytes)\n", outPath, len(f.Bytes()))
	return nil
}

// cmdRpoExtract implementA "advplc rpo extract <rpo> [--auto]"
// - Sem --auto: carrega dump prévio (.extract.json ou .extract.funcs)
// - Com --auto: tenta rodar o script gdb automaticamente
func cmdRpoExtract(args []string, auto bool) error {
	rpoPath := args[0]

	// Primeiro, identificar o RPO
	data, err := os.ReadFile(rpoPath)
	if err != nil {
		return err
	}
	profile, err := rpo.Identify(data)
	if err != nil {
		return err
	}

	fmt.Println("=== RPO Identificado ===")
	fmt.Print(profile.FormatSummary())
	fmt.Println()

	if auto {
		return rpoExtractAuto(profile, rpoPath)
	}

	// Modo normal: carregar dump existente
	result, err := rpo.ExtractFromRPO(rpoPath)
	if err != nil {
		// Sugerir o comando de extração
		fmt.Println("Nenhum dump encontrado. Para extrair funções:")
		fmt.Println()
		fmt.Println(profile.SuggestExtractCommand(rpoPath))
		return err
	}

	fmt.Printf("RPO ..........: %s\n", rpoPath)
	fmt.Printf("Total apois ..: %d\n", result.Count)
	fmt.Printf("Funções ......: %d\n", len(result.Functions))
	fmt.Println()
	if len(result.Functions) > 0 {
		fmt.Println("--- Funções ---")
		for _, f := range result.Functions {
			fmt.Printf("  %s\n", f)
		}
	} else {
		fmt.Println("(nenhuma função encontrada no dump)")
	}
	return nil
}

// rpoExtractContainer resolve o nome do container Docker onde o appserver
// roda, permitindo override via ADVPP_RPO_CONTAINER (necessário porque cada
// versão do Protheus usa um container próprio — ex.: "protheus-compile" para
// a matriz padrão, "protheus-compile-12.1.2510" para a 12.1.2510).
func rpoExtractContainer() string {
	if c := os.Getenv("ADVPP_RPO_CONTAINER"); c != "" {
		return c
	}
	return "protheus-compile"
}

// rpoExtractAppDir resolve o diretório do appserver dentro do container,
// permitindo override via ADVPP_RPO_APPDIR (o caminho muda entre versões do
// Protheus — ex.: "/protheus12/bin/appserver" na 12.1.2510).
func rpoExtractAppDir() string {
	if d := os.Getenv("ADVPP_RPO_APPDIR"); d != "" {
		return d
	}
	return "/protheus12/bin/appserver"
}

// rpoExtractApoDir resolve o diretório "apo" (onde ficam os .rpo e os fontes
// compilados), permitindo override via ADVPP_RPO_APODIR.
func rpoExtractApoDir() string {
	if d := os.Getenv("ADVPP_RPO_APODIR"); d != "" {
		return d
	}
	return "/protheus12/apo"
}

// rpoExtractEnv resolve o nome do ambiente (`-env=`) usado no -compile de
// gatilho, permitindo override via ADVPP_RPO_ENV (varia por instalação —
// ex.: "P12" na matriz padrão, "environment" na 12.1.2510).
func rpoExtractEnv() string {
	if e := os.Getenv("ADVPP_RPO_ENV"); e != "" {
		return e
	}
	return "P12"
}

// rpoExtractAuto extrai a lista de funções de um RPO rodando um appserver
// real dentro de um container Docker. Como o conteúdo do RPO é
// criptografado (ver docs/rpo-format.md, Fase 8), a única forma de obter a
// lista de funções é forçar o próprio appserver a carregar e regravar o RPO
// (`-compile` de um fonte-gatilho trivial no mesmo diretório/ambiente) e
// interceptar a chamada de `tAppMap::EndBuild()` via gdb — ver
// tools/rpo-live-inspect/extract_rpo.py.
func rpoExtractAuto(profile *rpo.RPOProfile, rpoPath string) error {
	container := rpoExtractContainer()
	appDir := rpoExtractAppDir()
	apoDir := rpoExtractApoDir()
	envName := rpoExtractEnv()

	scriptPath, err := filepath.Abs("tools/rpo-live-inspect/extract_rpo.py")
	if err != nil {
		return fmt.Errorf("não foi possível resolver o caminho do script: %w", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script de extração não encontrado: %s", scriptPath)
	}

	rpoAbs, err := filepath.Abs(rpoPath)
	if err != nil {
		return err
	}

	// O RPO precisa ficar no diretório "apo", com o NOME que o appserver
	// espera (profile.Name, lido do próprio container do arquivo — ex.:
	// "custom", "tttm120", "tlpp"), senão o -compile de gatilho não o toca.
	targetRPO := fmt.Sprintf("%s/%s.rpo", apoDir, profile.Name)
	fmt.Printf("Copiando script e RPO para o container %q (destino: %s)...\n", container, targetRPO)
	if out, err := exec.Command("docker", "cp", scriptPath, container+":/tmp/extract_rpo.py").CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp (script) falhou: %w\n%s", err, out)
	}
	if out, err := exec.Command("docker", "cp", rpoAbs, container+":"+targetRPO).CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp (rpo) falhou: %w\n%s", err, out)
	}

	// Fonte-gatilho trivial — só existe para forçar o appserver a abrir e
	// regravar o RPO alvo, expondo tanto as funções antigas quanto esta.
	triggerFunc := "RPOEXT01"
	triggerSrc := fmt.Sprintf("%s/rpoextract_trigger.prw", apoDir)
	triggerContent := fmt.Sprintf("User Function %s()\nReturn .T.\n", triggerFunc)
	writeCmd := exec.Command("docker", "exec", container, "bash", "-c",
		fmt.Sprintf("cat > %s << 'EOF'\n%sEOF", triggerSrc, triggerContent))
	if out, err := writeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("não foi possível gravar o fonte-gatilho: %w\n%s", err, out)
	}

	fmt.Printf("Rodando extração gdb (%s, env=%s)...\n", profile.Type, envName)
	gdbCmd := exec.Command("docker", "exec", container,
		"bash", "-c",
		fmt.Sprintf(
			"cd %s && export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH LANG=C.UTF-8 LC_ALL=C.UTF-8 PYTHONIOENCODING=utf-8 && "+
				"timeout 120 gdb -q -batch -x /tmp/extract_rpo.py --args ./appsrvlinux -compile -env=%s -files=%s -includes=%s 2>&1",
			appDir, envName, triggerSrc, apoDir,
		),
	)
	if out, err := gdbCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gdb extraction failed: %w\n%s", err, out)
	} else {
		fmt.Println(string(out))
	}

	// Copiar resultado de volta
	fmt.Println("Copiando resultado...")
	resultCmd := exec.Command("docker", "cp",
		container+":/tmp/rpo_extract.log.json",
		rpoPath+".extract.json",
	)
	if out, err := resultCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("não foi possível copiar o resultado: %w\n%s", err, out)
	}

	// Carregar e mostrar
	result, err := rpo.ParseExtractJSON(rpoPath + ".extract.json")
	if err != nil {
		return err
	}

	fmt.Printf("\n=== Resultado da Extração ===\n")
	fmt.Printf("Total apois ..: %d\n", result.Count)
	fmt.Printf("Funções ......: %d\n", len(result.Functions))
	if len(result.Functions) > 0 {
		fmt.Println("--- Top 20 funções ---")
		for i, f := range result.Functions {
			if i >= 20 {
				break
			}
			fmt.Printf("  %s\n", f)
		}
		if len(result.Functions) > 20 {
			fmt.Printf("  ... e mais %d funções\n", len(result.Functions)-20)
		}
	}
	return nil
}

