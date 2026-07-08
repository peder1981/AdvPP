package preprocessor

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Preprocessor struct {
	includePaths []string
	defines      map[string]string
	processed    map[string]bool
	sqlCounter   int
}

func NewPreprocessor(includePaths []string) *Preprocessor {
	return &Preprocessor{
		includePaths: includePaths,
		defines:      make(map[string]string),
		processed:    make(map[string]bool),
	}
}

func (p *Preprocessor) Process(source, fileName string) (string, error) {
	return p.processFile(source, fileName, 0)
}

func (p *Preprocessor) processFile(source, fileName string, depth int) (string, error) {
	if depth > 30 {
		return "", nil
	}

	lines := strings.Split(source, "\n")
	var output strings.Builder

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "#INCLUDE") {
			includeFile := extractIncludeFile(trimmed)
			if includeFile != "" {
				incSource, err := p.loadInclude(includeFile)
				if err == nil && incSource != "" {
					processed, _ := p.processFile(incSource, includeFile, depth+1)
					output.WriteString(processed)
					output.WriteString("\n")
				}
			}
			i++
			continue
		}

		if strings.HasPrefix(upper, "#DEFINE") {
			p.parseDefine(trimmed)
			i++
			continue
		}

		if strings.HasPrefix(upper, "#UNDEFINE") || strings.HasPrefix(upper, "#UNDEF") {
			name := extractDefineName(trimmed)
			delete(p.defines, name)
			i++
			continue
		}

		if strings.HasPrefix(upper, "#IFDEF") || strings.HasPrefix(upper, "#IFNDEF") {
			isIfDef := strings.HasPrefix(upper, "#IFDEF")
			defineName := strings.TrimSpace(trimmed[6:])
			if !isIfDef {
				defineName = strings.TrimSpace(trimmed[7:])
			}
			_, defined := p.defines[defineName]

			condMatch := isIfDef == defined

			var thenLines, elseLines []string
			inElse := false
			nested := 1
			i++
			for i < len(lines) && nested > 0 {
				innerUpper := strings.ToUpper(strings.TrimSpace(lines[i]))
				if strings.HasPrefix(innerUpper, "#IFDEF") || strings.HasPrefix(innerUpper, "#IFNDEF") {
					nested++
				}
				if strings.HasPrefix(innerUpper, "#ENDIF") {
					nested--
					if nested == 0 {
						i++
						break
					}
				}
				if nested == 1 && strings.HasPrefix(innerUpper, "#ELSE") {
					inElse = true
					i++
					continue
				}
				if inElse {
					elseLines = append(elseLines, lines[i])
				} else {
					thenLines = append(thenLines, lines[i])
				}
				i++
			}

			// Recurse through the normal pipeline instead of writing the
			// picked branch's raw lines straight to output — otherwise
			// anything needing its own processing inside the guard
			// (nested #include, #define, and critically the #xcommand/
			// #xtranslate multi-line-continuation skip above) never gets
			// it. This was silently corrupting real code: a `#xtranslate
			// ... ;` template inside an `#ifndef HEADER_GUARD` block
			// (extremely common) had its raw pattern-template text,
			// escaped braces and all, fed straight to the lexer.
			branch := thenLines
			if !condMatch {
				branch = elseLines
			}
			processed, err := p.processFile(strings.Join(branch, "\n"), fileName, depth+1)
			if err == nil {
				output.WriteString(processed)
				output.WriteString("\n")
			}
			continue
		}

		if strings.HasPrefix(upper, "#ENDIF") || strings.HasPrefix(upper, "#ELSE") {
			i++
			continue
		}

		if strings.HasPrefix(upper, "#XCOMMAND") || strings.HasPrefix(upper, "#XTRANSLATE") ||
			strings.HasPrefix(upper, "#COMMAND") || strings.HasPrefix(upper, "#TRANSLATE") {
			i++
			// These command-translation templates commonly span multiple
			// physical lines via a trailing ';' continuation (same
			// convention as regular code). Without following it, a
			// continuation line — plain pattern-template text, not a line
			// that itself starts with '#' — falls through to the normal
			// code path below and gets fed to the lexer as if it were real
			// AdvPL, escaped braces (`\{`/`\}`) and all.
			for strings.HasSuffix(strings.TrimRight(trimmed, " \t"), ";") && i < len(lines) {
				trimmed = strings.TrimSpace(lines[i])
				i++
			}
			continue
		}

		if upper == "BEGINSQL" || strings.HasPrefix(upper, "BEGINSQL ") {
			i++
			alias := extractSqlAlias(trimmed)
			var sqlLines []string
			for i < len(lines) {
				lu := strings.ToUpper(strings.TrimSpace(lines[i]))
				if lu == "ENDSQL" || strings.HasPrefix(lu, "ENDSQL ") {
					i++
					break
				}
				sqlLines = append(sqlLines, lines[i])
				i++
			}
			output.WriteString(p.renderSqlBlock(alias, sqlLines))
			continue
		}

		processed := p.applyDefines(line)
		output.WriteString(processed)
		output.WriteString("\n")
		i++
	}

	return output.String(), nil
}

var sqlAliasRe = regexp.MustCompile(`(?i)\bALIAS\s+(\w+)`)

func extractSqlAlias(header string) string {
	m := sqlAliasRe.FindStringSubmatch(header)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

var sqlMacroRe = regexp.MustCompile(`%(\w+)(?::([^%]*))?%`)

// renderSqlBlock turns a BeginSql...EndSql block into plain AdvPL: a string
// built line by line via '+=', with %Exp:x% substituting the real AdvPL
// expression x (the one macro that matters for correctness) and every other
// %macro% (Table, xFilial, Notdel, ...) dropped — this interpreter has no
// SQL engine to feed the query to, so the goal is a valid, parseable
// stand-in rather than a runnable query.
func (p *Preprocessor) renderSqlBlock(alias string, sqlLines []string) string {
	p.sqlCounter++
	varName := "__sql" + strconv.Itoa(p.sqlCounter)

	var out strings.Builder
	out.WriteString(varName + " := \"\"\n")
	for _, line := range sqlLines {
		content := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(content) == "" {
			continue
		}
		out.WriteString(varName + " += " + sqlLineToExpr(content) + "\n")
	}
	if alias != "" {
		out.WriteString(alias + " := " + varName + "\n")
	}
	return out.String()
}

// sqlLineToExpr renders one BeginSql body line as an AdvPL string
// expression, splitting out %Exp:ident% into real +(ident)+ segments.
func sqlLineToExpr(line string) string {
	var parts []string
	last := 0
	for _, m := range sqlMacroRe.FindAllStringSubmatchIndex(line, -1) {
		lit := line[last:m[0]]
		parts = append(parts, quoteSqlLit(lit))
		name := line[m[2]:m[3]]
		if strings.EqualFold(name, "EXP") && m[4] >= 0 {
			arg := strings.TrimSpace(line[m[4]:m[5]])
			parts = append(parts, "("+arg+")")
		}
		last = m[1]
	}
	parts = append(parts, quoteSqlLit(line[last:]))
	return strings.Join(parts, " + ")
}

func quoteSqlLit(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	return `"` + s + `"`
}

var includeRe = regexp.MustCompile(`#include\s+[<"]([^>"]+)[>"]`)

func extractIncludeFile(line string) string {
	matches := includeRe.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func extractDefineName(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (p *Preprocessor) parseDefine(line string) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) >= 2 {
		name := strings.TrimSpace(parts[1])
		value := ""
		if len(parts) >= 3 {
			value = strings.TrimSpace(stripTrailingLineComment(parts[2]))
		}
		p.defines[name] = value
	}
}

// stripTrailingLineComment removes a `// ...` tail from a #define value.
// Without this, a very common real-world style (`#Define X "1" + Y // why`)
// stores the comment text as part of the macro's replacement value — every
// later use of X then injects a stray `//` mid-line, silently swallowing
// the rest of that physical line (this was the root cause of several
// "only fails inside a large real file" parser bugs). Quote-aware so a
// genuine `//` inside a string value (e.g. a URL) isn't mistaken for one.
func stripTrailingLineComment(s string) string {
	inString := byte(0)
	for i := 0; i < len(s)-1; i++ {
		ch := s[i]
		if inString != 0 {
			if ch == inString {
				inString = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inString = ch
			continue
		}
		if ch == '/' && s[i+1] == '/' {
			return s[:i]
		}
	}
	return s
}

func (p *Preprocessor) applyDefines(line string) string {
	for name, value := range p.defines {
		line = replaceWord(line, name, value)
	}
	return line
}

func replaceWord(line, old, new string) string {
	if old == "" {
		return line
	}
	var result strings.Builder
	i := 0
	for i < len(line) {
		idx := strings.Index(line[i:], old)
		if idx == -1 {
			result.WriteString(line[i:])
			break
		}
		actualIdx := i + idx
		beforeOK := actualIdx == 0 || !isWordChar(line[actualIdx-1])
		afterIdx := actualIdx + len(old)
		afterOK := afterIdx >= len(line) || !isWordChar(line[afterIdx])
		if beforeOK && afterOK {
			result.WriteString(line[i:actualIdx])
			result.WriteString(new)
			i = afterIdx
		} else {
			result.WriteString(line[i : actualIdx+1])
			i = actualIdx + 1
		}
	}
	return result.String()
}

func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func (p *Preprocessor) loadInclude(fileName string) (string, error) {
	key := strings.ToLower(fileName)
	if p.processed[key] {
		return "", nil
	}
	p.processed[key] = true

	for _, dir := range p.includePaths {
		path := filepath.Join(dir, fileName)
		data, err := os.ReadFile(path)
		if err == nil && !isBinary(data) {
			return string(data), nil
		}
	}

	data, err := os.ReadFile(fileName)
	if err == nil && !isBinary(data) {
		return string(data), nil
	}

	return "", nil
}

// isBinary reports whether data looks like a compiled/compressed header
// (e.g. TOTVS "#zip"-packed .ch files) rather than plain AdvPL/TLPP source.
// Such includes can't be preprocessed as text, so callers should treat them
// as unavailable rather than feeding raw bytes into the lexer.
func isBinary(data []byte) bool {
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	for _, b := range sample {
		if b == 0 {
			return true
		}
	}
	return false
}

func (p *Preprocessor) GetDefines() map[string]string {
	return p.defines
}
