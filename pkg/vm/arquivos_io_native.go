package vm

// implementação das 29 funções de "Manipulacao-de-arquivos-discos-IO" (TDN),
// registradas via registerArquivosioNatives. Arquivo novo — não altera
// natives.go. Conflitos verificados por grep: nenhum dos 29 nomes existia.
//
// Notas arquiteturais:
//   - Parâmetros por referência (`@aNomesArq`, `@aUnits`, `@cBufferVar`,
//     `@nRet`, `@nFilesOut`) não são graváveis nesta VM. Nas funções com
//     canal de saída exclusivamente por `@` (ADir, FListZip, ListDrives,
//     TarDecomp), o resultado é devolvido como valor de retorno e o gap é
//     documentado em cada função.
//   - Diálogos (CGETFILE/TFILEDIALOG): sem GUI, retornam "" (cancelado),
//     conforme a spec exige interação do usuário (SmartClient/WebApp).
//   - Em erro sempre é devolvido advplrt.Nil (ou o código/.F. documentado
//     pela função), nunca erro Go como segundo retorno.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// ---------------------------------------------------------------------------
// Helpers locais (sem colisão com o restante do pacote)
// ---------------------------------------------------------------------------

// optStr retorna o argumento idx convertido para string, ou "" se ausente/Nil.
func optStr(args []advplrt.Value, idx int) string {
	if idx < len(args) && !advplrt.IsNil(args[idx]) {
		return advplrt.ToString(args[idx])
	}
	return ""
}

// optBool retorna o argumento idx como bool, ou def se ausente/Nil.
func optBool(args []advplrt.Value, idx int, def bool) bool {
	if idx < len(args) && !advplrt.IsNil(args[idx]) {
		return advplrt.ToBool(args[idx])
	}
	return def
}

// optNum retorna o argumento idx como número, ou def se ausente/Nil.
func optNum(args []advplrt.Value, idx int, def float64) float64 {
	if idx < len(args) && !advplrt.IsNil(args[idx]) {
		return advplrt.ToFloat(args[idx])
	}
	return def
}

// normPath converte barras invertidas (Windows) em barras normais e remove a
// letra de unidade ("c:" / "l:"), como especificado pelas funções para Linux.
func normPath(p string) string {
	if p == "" {
		return p
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}
	return p
}

// nilValue aplica o NILVALUE ("-") do RFC 5424 a campos vazios do LogMsg.
func nilValue(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// fileItem representa um arquivo/diretório listado por DIRECTORY/ADIR.
type fileItem struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

// maskMatches aplica a máscara wildcard ao nome. *.* / * / ** são considerados
// match-all (comportamento DOS esperado pelo AdvPL).
func maskMatches(mask, name string) bool {
	if mask == "" || mask == "*" || mask == "*.*" || mask == "**" {
		return true
	}
	ok, err := filepath.Match(mask, name)
	return err == nil && ok
}

// splitSpec separa cDirEsp em (baseDir, relPattern), onde baseDir é o maior
// prefixo sem wildcards e relPattern é o restante (pode conter wildcards).
// Sem wildcards, devolve (spec, "").
func splitSpec(spec string) (baseDir, relPattern string) {
	comps := strings.Split(spec, "/")
	var base []string
	i := 0
	found := false
	for ; i < len(comps); i++ {
		if strings.ContainsAny(comps[i], "*?[") {
			found = true
			break
		}
		if comps[i] != "" || i == 0 {
			base = append(base, comps[i])
		}
	}
	if !found {
		return spec, ""
	}
	b := strings.Join(base, "/")
	if b == "" {
		b = "."
	}
	return b, strings.Join(comps[i:], "/")
}

// listDirSpec lista as entradas do diretório/wildcard cDirEsp, sem filtros de
// atributo (aplicados pelos chamadores).
func listDirSpec(spec string) ([]fileItem, error) {
	spec = normPath(spec)
	base, rel := splitSpec(spec)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var out []fileItem
	for _, e := range entries {
		if rel != "" && !maskMatches(rel, e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, fileItem{
			name:    e.Name(),
			size:    info.Size(),
			modTime: info.ModTime(),
			isDir:   e.IsDir(),
		})
	}
	return out, nil
}

// filterItems aplica os flags de atributo (H=escondidos, D=diretórios).
// S (sistema) não é distinguível no Linux e "V" (volume) é tratado pelo
// chamador (sem suporte a volume DOS nesta VM).
func filterItems(items []fileItem, includeHidden, includeDirs, includeSystem bool) []fileItem {
	var out []fileItem
	for _, it := range items {
		if !includeHidden && strings.HasPrefix(it.name, ".") {
			continue
		}
		if it.isDir {
			if includeDirs || includeSystem {
				out = append(out, it)
			}
			continue
		}
		out = append(out, it)
	}
	return out
}

// sortItems ordena conforme nTypeOrder (1=nome, 2=data, 3=tamanho).
func sortItems(items []fileItem, order int) {
	switch order {
	case 2:
		sort.SliceStable(items, func(i, j int) bool { return items[i].modTime.Before(items[j].modTime) })
	case 3:
		sort.SliceStable(items, func(i, j int) bool { return items[i].size < items[j].size })
	default:
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})
	}
}

// splitOffset extrai o sufixo ":N" (a partir da build 7.00.131227A-20160630,
// listagem limitada a 10.000 itens por janela) de uma spec de arquivo/atributos.
func splitOffset(s string) (string, int) {
	idx := strings.LastIndex(s, ":")
	if idx > 0 {
		if n, err := strconv.Atoi(s[idx+1:]); err == nil {
			return s[:idx], n
		}
	}
	return s, 0
}

// octalToFileMode interpreta a representação decimal de nFileMode como octal
// (ex.: 666 -> 0o666), como documentado pela CHMOD.
func octalToFileMode(n float64) os.FileMode {
	s := strconv.FormatFloat(n, 'f', 0, 64)
	val, err := strconv.ParseInt(s, 8, 32)
	if err != nil || val < 0 {
		return os.FileMode(int(n)).Perm()
	}
	return os.FileMode(val).Perm()
}

// copyFileData copia src -> dst usando buffer de bufSize bytes (0 = padrão).
func copyFileData(src, dst string, bufSize int) error {
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	buf := make([]byte, bufSize)
	_, err = io.CopyBuffer(out, in, buf)
	return err
}

// copyFilePreserveTimes copia src -> dst e preserva o datetime de origem.
func copyFilePreserveTimes(src, dst string) error {
	if err := copyFileData(src, dst, 0); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// folderDest une cFolder + basename(cFile), para as cópias que recebem pasta.
func folderDest(file, folder string) string {
	return filepath.Join(normPath(folder), filepath.Base(normPath(file)))
}

// gzipFile compacta um arquivo único para o formato gzip (GNU zip).
func gzipFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

// addFileToZip adiciona um arquivo (ou diretório, recursivamente) ao zip.
func addFileToZip(zw *zip.Writer, absPath, name string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return addDirToZip(zw, absPath, name)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// addDirToZip grava a entrada do diretório e percorre o conteúdo.
func addDirToZip(zw *zip.Writer, dirPath, prefix string) error {
	prefix = strings.TrimSuffix(prefix, "/")
	if _, err := zw.Create(prefix + "/"); err != nil {
		return err
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := filepath.Join(dirPath, e.Name())
		name := prefix + "/" + e.Name()
		if err := addFileToZip(zw, child, name); err != nil {
			return err
		}
	}
	return nil
}

// sanitizeZipName limpa nomes vindos de arquivos compactados, impedindo
// escape de diretório (zip-slip).
func sanitizeZipName(name string) (string, bool) {
	name = filepath.ToSlash(name)
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return strings.TrimPrefix(clean, "/"), true
}

// extractZipTo extrai todos os arquivos do zip para folder.
func extractZipTo(zipFile, folder string) error {
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		name, ok := sanitizeZipName(f.Name)
		if !ok {
			continue
		}
		dest := filepath.Join(folder, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// xorBytes aplica ofuscação XOR com a senha (formato MZP proprietário desta
// VM). Não é criptografia forte: documentado como limitação frente ao padrão
// Microsiga real.
func xorBytes(data []byte, pass string) []byte {
	if len(pass) == 0 {
		return data
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ pass[i%len(pass)]
	}
	return out
}

// mzpCompress monta o payload MZP (nome + tamanho + conteúdo por arquivo),
// compacta com zlib (RFC 1950) e aplica XOR com a senha (se houver).
func mzpCompress(files []string, cPass string) ([]byte, error) {
	var payload bytes.Buffer
	payload.WriteString("MZP1")
	_ = binary.Write(&payload, binary.LittleEndian, uint32(len(files)))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(normPath(f))
		_ = binary.Write(&payload, binary.LittleEndian, uint32(len(name)))
		payload.WriteString(name)
		_ = binary.Write(&payload, binary.LittleEndian, uint32(len(data)))
		payload.Write(data)
	}
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	if _, err := zw.Write(payload.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return xorBytes(out.Bytes(), cPass), nil
}

// tarAddRecursive adiciona arquivo ou diretório (recursivo) ao tar.
func tarAddRecursive(tw *tar.Writer, absPath, name string) error {
	fi, err := os.Lstat(absPath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = strings.TrimSuffix(name, "/") + "/"
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := tarAddRecursive(tw, filepath.Join(absPath, e.Name()), name+"/"+e.Name()); err != nil {
				return err
			}
		}
		return nil
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

// ---------------------------------------------------------------------------
// Estado global do driver de texto FT_* (FT_FUse/FT_FEOF/FT_FGoto)
// ---------------------------------------------------------------------------

var (
	ftMu         sync.Mutex
	ftCurrent    *ftTextFile
	ftNextHandle int
)

type ftTextFile struct {
	handle int
	file   *os.File
	pos    int64
}

func ftCloseCurrent() {
	ftMu.Lock()
	defer ftMu.Unlock()
	if ftCurrent != nil {
		ftCurrent.file.Close()
		ftCurrent = nil
	}
}

// ---------------------------------------------------------------------------
// Registro dos natives
// ---------------------------------------------------------------------------

func (v *VM) registerArquivosioNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ADir([cEspecArq], [@aNomesArq], [@aTamanhos], [@aDatas], [@aHoras],
	//      [@aAtributos], [lChangeCase]) -> nRet
	// Os arrays de saída são parâmetros por referência (não graváveis nesta
	// VM); retorna o array de nomes como valor (forma suportada de uso),
	// incluindo os aliases "." e ".." quando aAtributos é informado.
	natives["ADIR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSpec := ""
		if !advplrt.IsNil(getArg(args, 0)) {
			cSpec = advplrt.ToString(getArg(args, 0))
		}
		if cSpec == "" {
			cSpec = "*.*"
		}
		lChangeCase := optBool(args, 6, true)
		includeAttrs := !advplrt.IsNil(getArg(args, 5))

		cSpec, offset := splitOffset(cSpec)

		items, err := listDirSpec(cSpec)
		if err != nil {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		items = filterItems(items, includeAttrs, includeAttrs, includeAttrs)
		sortItems(items, 1)

		change := func(s string) string {
			if lChangeCase {
				return strings.ToLower(s)
			}
			return s
		}

		names := make([]advplrt.Value, 0, len(items)+2)
		if includeAttrs {
			names = append(names, advplrt.NewString(change(".")), advplrt.NewString(change("..")))
		}
		for _, it := range items {
			names = append(names, advplrt.NewString(change(it.name)))
		}
		if offset > 0 {
			if offset > len(names) {
				offset = len(names)
			}
			names = names[offset:]
		}
		if len(names) > 10000 {
			names = names[:10000]
		}
		return advplrt.NewArray(names), nil
	}

	// cGetFile([cMascara], [cTitulo], [nMascpadrao], [cDirinicial], [lAbrir],
	//          [nOpcoes], [lArvore], [lKeepCase]) -> cRet
	// Diálogo de seleção de arquivo exige SmartClient/WebApp com GUI. Sem GUI
	// nesta VM, retorna "" (nenhum item selecionado / usuário cancelou).
	natives["CGETFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}

	// CHMOD(cFileName, nFileMode, [uParam3], [lChangeCase]) -> lRet
	natives["CHMOD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		p := normPath(getArgString(args, 0, ""))
		mode := octalToFileMode(optNum(args, 1, 0))
		err := os.Chmod(p, mode)
		return advplrt.NewBool(err == nil), nil
	}

	// CpyF2Web(cOrigem, [lIsUserDiskDir], [lCompactCopy], [lChangeCase],
	//          [lUnZipFile]) -> cRet
	// Não há WebApp nesta VM: simula a cópia para a pasta temporária web
	// (os.TempDir()/advpp-web) e retorna o destino, ou "" em falha.
	natives["CPYF2WEB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		lUnZip := optBool(args, 4, false)
		root := filepath.Join(os.TempDir(), "advpp-web")

		info, err := os.Stat(src)
		if err != nil {
			return advplrt.NewString(""), nil
		}
		if info.IsDir() {
			return advplrt.NewString(""), nil
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return advplrt.NewString(""), nil
		}
		if lUnZip && strings.HasSuffix(strings.ToLower(src), ".zip") {
			if err := extractZipTo(src, root); err != nil {
				return advplrt.NewString(""), nil
			}
			return advplrt.NewString(root), nil
		}
		dst := filepath.Join(root, filepath.Base(src))
		if err := copyFileData(src, dst, 0); err != nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(dst), nil
	}

	// CpyS2T(cFile, cFolder, [lCompress], [lChangeCase], [nLenBuffer]) -> lRet
	// Server -> "client": copia cFile para a pasta cFolder no servidor local.
	// A compactação automática (lCompress) não é aplicada (sem transporte de
	// rede envolvido).
	natives["CPYS2T"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		folder := normPath(getArgString(args, 1, ""))
		nBuf := int(optNum(args, 4, 0))
		err := copyFileData(src, folderDest(src, folder), nBuf)
		return advplrt.NewBool(err == nil), nil
	}

	// CpyS2TEx(cServer, cClient, [lChangeCase]) -> lRet
	// Copia preservando o datetime de origem; o destino é caminho completo
	// (pode sobrescrever).
	natives["CPYS2TEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		dst := normPath(getArgString(args, 1, ""))
		err := copyFilePreserveTimes(src, dst)
		return advplrt.NewBool(err == nil), nil
	}

	// CpyS2TW(cOrigem, [lSendToBrowser]) -> nRet
	// Códigos da spec: 0 sucesso, -1 diretório já não é diretório/server,
	// -2 arquivo não existe no servidor, -3 falha de transmissão.
	natives["CPYS2TW"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		if src == "" {
			return advplrt.NewNumber(-2), nil
		}
		info, err := os.Stat(src)
		if err != nil {
			return advplrt.NewNumber(-2), nil
		}
		if info.IsDir() {
			return advplrt.NewNumber(-1), nil
		}
		dst := filepath.Join(os.TempDir(), "advpp-web", filepath.Base(src))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return advplrt.NewNumber(-3), nil
		}
		if err := copyFileData(src, dst, 0); err != nil {
			return advplrt.NewNumber(-3), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// CpyT2S(cFile, cFolder, [lCompress], [lChangeCase], [nLenBuffer]) -> lRet
	natives["CPYT2S"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		folder := normPath(getArgString(args, 1, ""))
		nBuf := int(optNum(args, 4, 0))
		err := copyFileData(src, folderDest(src, folder), nBuf)
		return advplrt.NewBool(err == nil), nil
	}

	// Directory(cDirEsp, [cAtributos], [uParam1], [lConvertCase],
	//           [nTypeOrder]) -> aRet
	// Retorna array de arrays [nome, tamanho, data, hora, atributos].
	natives["DIRECTORY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSpec := getArgString(args, 0, "*.*")
		cAttrs := getArgString(args, 1, "")
		lConvertCase := optBool(args, 3, true)
		nOrder := int(optNum(args, 4, 1))

		includeHidden := strings.Contains(cAttrs, "H")
		includeSystem := strings.Contains(cAttrs, "S")
		includeDirs := strings.Contains(cAttrs, "D")
		volume := strings.Contains(cAttrs, "V")

		cSpec, offset := splitOffset(cSpec)

		// "V": procura pelo volume DOS — sem suporte no Linux, retorna vazio.
		if volume {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		items, err := listDirSpec(cSpec)
		if err != nil {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		items = filterItems(items, includeHidden, includeDirs, includeSystem)
		sortItems(items, nOrder)

		if offset > 0 {
			if offset > len(items) {
				offset = len(items)
			}
			items = items[offset:]
		}
		if len(items) > 10000 {
			items = items[:10000]
		}

		out := make([]advplrt.Value, 0, len(items))
		for _, it := range items {
			name := it.name
			if lConvertCase {
				name = strings.ToUpper(name)
			}
			attr := "A"
			if it.isDir {
				attr = "D"
			}
			out = append(out, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(name),
				advplrt.NewNumber(float64(it.size)),
				advplrt.NewDate(it.modTime),
				advplrt.NewString(it.modTime.Format("15:04:05")),
				advplrt.NewString(attr),
			}))
		}
		return advplrt.NewArray(out), nil
	}

	// DirRemove(cPath, [uParam2], [lChangeCase]) -> lRet
	// A spec exige diretório vazio: usa os.Remove (falha se houver conteúdo).
	natives["DIRREMOVE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		p := normPath(getArgString(args, 0, ""))
		err := os.Remove(p)
		return advplrt.NewBool(err == nil), nil
	}

	// ExistDir(cPath, [uParam2], [lChangeCase]) -> lRet
	natives["EXISTDIR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		p := normPath(getArgString(args, 0, ""))
		info, err := os.Stat(p)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(info.IsDir()), nil
	}

	// FListZip(cZipFile, [@nRet], [cPassword], [lChangeCase]) -> aRet
	// Retorna array de arrays [nome, tamanho]; nRet é por referência e não é
	// gravável nesta VM (o array retornado já denota sucesso/falha).
	natives["FLISTZIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		zipFile := normPath(getArgString(args, 0, ""))
		if getArgString(args, 2, "") != "" {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		zr, err := zip.OpenReader(zipFile)
		if err != nil {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		defer zr.Close()
		out := make([]advplrt.Value, 0, len(zr.File))
		for _, f := range zr.File {
			out = append(out, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(f.Name),
				advplrt.NewNumber(float64(f.UncompressedSize64)),
			}))
		}
		return advplrt.NewArray(out), nil
	}

	// FRead(nHandle, cBufferVar, nQtdBytes) -> nRet
	// Lê até nQtdBytes do handle (FOpen/FCreate). cBufferVar é parâmetro por
	// referência; nesta VM devolve a contagem de bytes lidos (nRet da spec).
	// Para obter o conteúdo como string, usar FReadStr.
	natives["FREAD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		h := int(optNum(args, 0, 0))
		nBytes := int(optNum(args, 2, 0))
		f, ok := v.fileHandles[h]
		if !ok {
			v.lastFError = 6
			return advplrt.NewNumber(0), nil
		}
		if nBytes <= 0 {
			return advplrt.NewNumber(0), nil
		}
		buf := make([]byte, nBytes)
		r, _ := f.Read(buf)
		v.lastFError = 0
		return advplrt.NewNumber(float64(r)), nil
	}

	// FRename(cArquivo, cNovoArq, [nParam3], [lChangeCase]) -> nRet (0/-1)
	natives["FRENAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		dst := normPath(getArgString(args, 1, ""))
		if src == "" || dst == "" {
			return advplrt.NewNumber(-1), nil
		}
		if err := os.Rename(src, dst); err != nil {
			return advplrt.NewNumber(-1), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// FRenameEx(cArquivo, cNovoArq, [nParam3]) -> nRet (0/-1)
	// Respeita o case do segundo parâmetro (sem ajuste de case).
	natives["FRENAMEEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		dst := normPath(getArgString(args, 1, ""))
		if src == "" || dst == "" {
			return advplrt.NewNumber(-1), nil
		}
		if err := os.Rename(src, dst); err != nil {
			return advplrt.NewNumber(-1), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// FT_FEOF() -> lRet (.T. no fim do arquivo texto corrente)
	natives["FT_FEOF"] = func(args []advplrt.Value) (advplrt.Value, error) {
		ftMu.Lock()
		cur := ftCurrent
		ftMu.Unlock()
		if cur == nil {
			return advplrt.NewBool(true), nil
		}
		var b [1]byte
		n, err := cur.file.ReadAt(b[:], cur.pos)
		return advplrt.NewBool(n == 0 || err == io.EOF), nil
	}

	// FT_FGoto(nPos) -> move o ponteiro do arquivo texto para posição absoluta
	natives["FT_FGOTO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nPos := int64(optNum(args, 0, 0))
		ftMu.Lock()
		if ftCurrent != nil {
			if nPos < 0 {
				nPos = 0
			}
			ftCurrent.pos = nPos
		}
		ftMu.Unlock()
		return advplrt.Nil, nil
	}

	// FT_FUse([cTXTFile]) -> nRet (handle) / -1 em falha; sem argumento fecha
	// o arquivo corrente.
	natives["FT_FUSE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := ""
		if !advplrt.IsNil(getArg(args, 0)) {
			cFile = advplrt.ToString(getArg(args, 0))
		}
		if cFile == "" {
			ftCloseCurrent()
			return advplrt.NewNumber(0), nil
		}
		f, err := os.Open(normPath(cFile))
		if err != nil {
			ftCloseCurrent()
			return advplrt.NewNumber(-1), nil
		}
		ftMu.Lock()
		if ftCurrent != nil {
			ftCurrent.file.Close()
		}
		ftNextHandle++
		ftCurrent = &ftTextFile{handle: ftNextHandle, file: f, pos: 0}
		h := ftCurrent.handle
		ftMu.Unlock()
		return advplrt.NewNumber(float64(h)), nil
	}

	// FUnZip(cZipFile, cFolder, [cPassword], [lChangeCase]) -> nRet (0/-1)
	natives["FUNZIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		zipFile := normPath(getArgString(args, 0, ""))
		folder := normPath(getArgString(args, 1, ""))
		// Senha (ZIP 2.0 / AES) não é suportada pelo archive/zip: falha honesta.
		if getArgString(args, 2, "") != "" {
			return advplrt.NewNumber(-1), nil
		}
		if err := extractZipTo(zipFile, folder); err != nil {
			return advplrt.NewNumber(-1), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// FZip(cZipFile, aFiles, [cBaseDir], [cPassword], [lChangeCase]) -> nRet
	natives["FZIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		zipFile := normPath(getArgString(args, 0, ""))
		aFiles := getArg(args, 1)
		baseDir := normPath(getArgString(args, 2, ""))
		if getArgString(args, 3, "") != "" {
			return advplrt.NewNumber(-1), nil
		}

		var files []string
		if arr, ok := aFiles.(*advplrt.ArrayValue); ok {
			for _, v := range arr.Elements {
				files = append(files, normPath(advplrt.ToString(v)))
			}
		} else if !advplrt.IsNil(aFiles) {
			files = append(files, normPath(advplrt.ToString(aFiles)))
		}
		if len(files) == 0 {
			return advplrt.NewNumber(-1), nil
		}

		zf, err := os.Create(zipFile)
		if err != nil {
			return advplrt.NewNumber(-1), nil
		}
		zw := zip.NewWriter(zf)
		ok := true
		for _, f := range files {
			name := f
			if baseDir != "" {
				if rel, err := filepath.Rel(baseDir, f); err == nil && !strings.HasPrefix(rel, "..") {
					name = filepath.ToSlash(rel)
				}
			} else {
				name = filepath.Base(f)
			}
			if err := addFileToZip(zw, f, name); err != nil {
				ok = false
				break
			}
		}
		if !ok {
			zw.Close()
			zf.Close()
			os.Remove(zipFile)
			return advplrt.NewNumber(-1), nil
		}
		if err := zw.Close(); err != nil {
			zf.Close()
			return advplrt.NewNumber(-1), nil
		}
		zf.Close()
		return advplrt.NewNumber(0), nil
	}

	// GzCompress(cFile, [cGzip], [lChangeCase]) -> lRet
	// Compacta um único arquivo para gzip. Nome distinto de GzStrComp
	// (string-based, já existente) — sem conflito de registro.
	natives["GZCOMPRESS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := normPath(getArgString(args, 0, ""))
		dst := normPath(getArgString(args, 1, ""))
		if dst == "" {
			dst = src + ".gz"
		}
		if src == "" {
			return advplrt.NewBool(false), nil
		}
		if err := gzipFile(src, dst); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// GzDecomp(cGzip, cOutDir, [lChangeCase]) -> lRet
	natives["GZDECOMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		gzPath := normPath(getArgString(args, 0, ""))
		outDir := normPath(getArgString(args, 1, ""))
		if gzPath == "" || outDir == "" {
			return advplrt.NewBool(false), nil
		}
		info, err := os.Stat(gzPath)
		if err != nil || info.IsDir() {
			return advplrt.NewBool(false), nil
		}
		f, err := os.Open(gzPath)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		defer f.Close()
		gr, err := gzip.NewReader(f)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		defer gr.Close()
		outName := gr.Name
		if outName == "" {
			outName = strings.TrimSuffix(filepath.Base(gzPath), ".gz")
		}
		if outName == "" {
			outName = "out"
		}
		data, err := io.ReadAll(gr)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return advplrt.NewBool(false), nil
		}
		if err := os.WriteFile(filepath.Join(outDir, filepath.Base(outName)), data, 0o644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// ListDrives([@aUnits], [@aTypes], nWhere) -> lRet
	// aUnits/aTypes são por referência (não graváveis): o array de unidades é
	// devolvido como valor de retorno. No Linux as montagens se resumem à
	// raiz "/". nWhere inválido retorna Nil ("Invalid nWhere parameter").
	natives["LISTDRIVES"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nWhere := int(optNum(args, 2, 0))
		if nWhere != 0 && nWhere != 1 {
			return advplrt.Nil, nil
		}
		if runtime.GOOS == "windows" {
			var drives []string
			for d := byte('A'); d <= 'Z'; d++ {
				if _, err := os.Stat(string([]byte{d}) + ":\\"); err == nil {
					drives = append(drives, string([]byte{d})+":")
				}
			}
			if len(drives) == 0 {
				drives = []string{"C:"}
			}
			out := make([]advplrt.Value, 0, len(drives))
			for _, d := range drives {
				out = append(out, advplrt.NewString(d))
			}
			return advplrt.NewArray(out), nil
		}
		return advplrt.NewArray([]advplrt.Value{advplrt.NewString("/")}), nil
	}

	// LogMsg(cFunc, nFacility, nSeverity, nVersao, cMsgId, cStrData,
	//        uMsg1, ...) -> .T.
	// Grava uma linha no formato SysLog (RFC 5424) em ADVPP_LOG (ou
	// os.TempDir()/advpp.log), de forma síncrona e best-effort.
	natives["LOGMSG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFunc := getArgString(args, 0, "")
		nFacility := int(optNum(args, 1, 0))
		nSeverity := int(optNum(args, 2, 0))
		nVersao := int(optNum(args, 3, 0))
		cMsgId := getArgString(args, 4, "")
		cStrData := getArgString(args, 5, "")
		msgs := make([]string, 0, len(args)-6)
		for i := 6; i < len(args); i++ {
			if !advplrt.IsNil(args[i]) {
				msgs = append(msgs, advplrt.ToString(args[i]))
			}
		}
		if nFacility < 0 || nFacility > 23 {
			nFacility = 0
		}
		if nSeverity < 0 || nSeverity > 7 {
			nSeverity = 0
		}
		if nVersao < 1 {
			nVersao = 1
		}
		logPath := os.Getenv("ADVPP_LOG")
		if logPath == "" {
			logPath = filepath.Join(os.TempDir(), "advpp.log")
		}
		host, _ := os.Hostname()
		pri := nFacility*8 + nSeverity
		stamp := time.Now().Format("2006-01-02T15:04:05Z07:00")
		line := fmt.Sprintf("<%d>%d %s %s %s %s %s %s %s",
			pri, nVersao, stamp, nilValue(host), nilValue(cFunc),
			nilValue(cMsgId), nilValue(cStrData), strings.TrimSpace(strings.Join(msgs, " ")), "")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return advplrt.NewBool(true), nil
		}
		defer f.Close()
		fmt.Fprintln(f, line)
		return advplrt.NewBool(true), nil
	}

	// MsCompress(xFile, [cDest], [cPass], [lChangeCase]) -> cRet
	// Formato MZP (Microsiga Zip) proprietário: payload zlib (RFC 1950) com
	// ofuscação XOR quando senha informada (não é o padrão Microsiga real).
	natives["MSCOMPRESS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		xFile := getArg(args, 0)
		var files []string
		switch tv := xFile.(type) {
		case *advplrt.ArrayValue:
			for _, v := range tv.Elements {
				files = append(files, normPath(advplrt.ToString(v)))
			}
		default:
			if !advplrt.IsNil(xFile) {
				files = append(files, normPath(advplrt.ToString(xFile)))
			}
		}
		if len(files) == 0 {
			return advplrt.NewString(""), nil
		}
		cDest := getArgString(args, 1, "")
		if cDest == "" {
			base := strings.TrimSuffix(filepath.Base(files[0]), filepath.Ext(files[0]))
			cDest = base + ".mzp"
		}
		if !strings.HasSuffix(strings.ToLower(cDest), ".mzp") {
			cDest += ".mzp"
		}
		cPass := getArgString(args, 2, "")
		data, err := mzpCompress(files, cPass)
		if err != nil {
			return advplrt.NewString(""), nil
		}
		if err := os.WriteFile(normPath(cDest), data, 0o644); err != nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(normPath(cDest)), nil
	}

	// MsDecomp(xFile, [cDest], [cPass]) -> lRet
	natives["MSDECOMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		xFile := normPath(getArgString(args, 0, ""))
		cDest := getArgString(args, 1, "")
		cPass := getArgString(args, 2, "")
		if xFile == "" {
			return advplrt.NewBool(false), nil
		}
		data, err := os.ReadFile(xFile)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		if cPass != "" {
			data = xorBytes(data, cPass)
		}
		if cDest == "" {
			cDest = "."
		}
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		payload, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		if len(payload) < 4 || string(payload[:4]) != "MZP1" {
			return advplrt.NewBool(false), nil
		}
		if err := os.MkdirAll(cDest, 0o755); err != nil {
			return advplrt.NewBool(false), nil
		}
		r := bytes.NewReader(payload[4:])
		var count uint32
		if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
			return advplrt.NewBool(false), nil
		}
		for i := uint32(0); i < count; i++ {
			var nl uint32
			if err := binary.Read(r, binary.LittleEndian, &nl); err != nil {
				return advplrt.NewBool(false), nil
			}
			nameB := make([]byte, nl)
			if _, err := io.ReadFull(r, nameB); err != nil {
				return advplrt.NewBool(false), nil
			}
			var cl uint32
			if err := binary.Read(r, binary.LittleEndian, &cl); err != nil {
				return advplrt.NewBool(false), nil
			}
			content := make([]byte, cl)
			if _, err := io.ReadFull(r, content); err != nil {
				return advplrt.NewBool(false), nil
			}
			dest := filepath.Join(cDest, filepath.Base(string(nameB)))
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				return advplrt.NewBool(false), nil
			}
		}
		return advplrt.NewBool(true), nil
	}

	// TarCompress(aItens, cDest, [lChangeCase]) -> cFile ("" em falha)
	natives["TARCOMPRESS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		aItens := getArg(args, 0)
		cDest := normPath(getArgString(args, 1, ""))
		arr, ok := aItens.(*advplrt.ArrayValue)
		if !ok || cDest == "" {
			return advplrt.NewString(""), nil
		}
		tf, err := os.Create(cDest)
		if err != nil {
			return advplrt.NewString(""), nil
		}
		tw := tar.NewWriter(tf)
		ok = true
		for _, v := range arr.Elements {
			p := normPath(advplrt.ToString(v))
			if _, err := os.Stat(p); err != nil {
				ok = false
				break
			}
			name := strings.TrimPrefix(filepath.ToSlash(p), "/")
			if err := tarAddRecursive(tw, p, name); err != nil {
				ok = false
				break
			}
		}
		if !ok {
			tw.Close()
			tf.Close()
			os.Remove(cDest)
			return advplrt.NewString(""), nil
		}
		if err := tw.Close(); err != nil {
			tf.Close()
			return advplrt.NewString(""), nil
		}
		tf.Close()
		abs, _ := filepath.Abs(cDest)
		return advplrt.NewString(abs), nil
	}

	// TarDecomp(cTarFile, cOutDir, [@nFilesOut], [lChangeCase]) -> lRet
	natives["TARDECOMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTar := normPath(getArgString(args, 0, ""))
		cOut := normPath(getArgString(args, 1, ""))
		if cOut == "" {
			return advplrt.NewBool(false), nil
		}
		tf, err := os.Open(cTar)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		defer tf.Close()
		tr := tar.NewReader(tf)
		if err := os.MkdirAll(cOut, 0o755); err != nil {
			return advplrt.NewBool(false), nil
		}
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return advplrt.NewBool(false), nil
			}
			name, ok := sanitizeZipName(hdr.Name)
			if !ok {
				continue
			}
			dest := filepath.Join(cOut, name)
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(dest, 0o755); err != nil {
					return advplrt.NewBool(false), nil
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return advplrt.NewBool(false), nil
				}
				f, err := os.Create(dest)
				if err != nil {
					return advplrt.NewBool(false), nil
				}
				if _, err := io.Copy(f, tr); err != nil {
					f.Close()
					return advplrt.NewBool(false), nil
				}
				f.Close()
			default:
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return advplrt.NewBool(false), nil
				}
			}
		}
		return advplrt.NewBool(true), nil
	}

	// tFileDialog([cMascara], [cTitulo], [nMascpadrao], [cDirinicial],
	//             [lSalvar], [nOpcoes]) -> cRet
	// Diálogo de seleção na estação de trabalho (SmartClient) com GUI. Sem GUI
	// nesta VM, retorna "" (exige interação humana; cDirinicial não é retorno
	// default neste diálogo).
	natives["TFILEDIALOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}
}