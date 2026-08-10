package vm

import (
	"os"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodoarquivoININatives registra funções de manipulação de arquivos INI:
// DeleteKeyINI, DeleteSectionINI, GetINISessions, GetPvProfileInt, GetPvProfString,
// GetSrvProfString, WriteSrvProfString.
func (v *VM) registerManipulacaodoarquivoININatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// DeleteKeyINI(cSecao, cChave, cIniFile) -> lRet
	natives["DELETEKEYINI"] = func(args []advplrt.Value) (advplrt.Value, error) {
		section := advplrt.ToString(getArg(args, 0))
		key := advplrt.ToString(getArg(args, 1))
		iniFile := advplrt.ToString(getArg(args, 2))

		success, err := iniDeleteKey(iniFile, section, key)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(success), nil
	}

	// DeleteSectionINI(cSecao, cIniFile) -> lRet
	natives["DELETESECTIONINI"] = func(args []advplrt.Value) (advplrt.Value, error) {
		section := advplrt.ToString(getArg(args, 0))
		iniFile := advplrt.ToString(getArg(args, 1))

		success, err := iniDeleteSection(iniFile, section)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(success), nil
	}

	// GetINISessions(cIni, [uParam1]) -> aRet (array of section names)
	natives["GETINISESSIONS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		iniFile := advplrt.ToString(getArg(args, 0))

		sections, err := iniGetSections(iniFile)
		if err != nil {
			// File not found or cannot be read; return empty array
			return advplrt.NewArray(nil), nil
		}

		arr := &advplrt.ArrayValue{}
		for _, section := range sections {
			arr.Elements = append(arr.Elements, advplrt.NewString(section))
		}
		return arr, nil
	}

	// GetPvProfileInt(cSecao, cChave, nPadrao, cNomeArqCfg, [uParam5], [uParam6]) -> nRet
	natives["GETPVPROFILEINT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		section := advplrt.ToString(getArg(args, 0))
		key := advplrt.ToString(getArg(args, 1))
		defaultVal := toNumber(getArg(args, 2))
		iniFile := advplrt.ToString(getArg(args, 3))

		val, err := iniGetKey(iniFile, section, key)
		if err != nil || val == "" {
			return advplrt.NewNumber(defaultVal), nil
		}

		// Try to parse as integer/numeric value
		n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return advplrt.NewNumber(defaultVal), nil
		}
		return advplrt.NewNumber(n), nil
	}

	// GetPvProfString(cSecao, cChave, cPadrao, cNomeArqCfg, [uParam5], [uParam6]) -> cRet
	natives["GETPVPROFSTRING"] = func(args []advplrt.Value) (advplrt.Value, error) {
		section := advplrt.ToString(getArg(args, 0))
		key := advplrt.ToString(getArg(args, 1))
		defaultVal := advplrt.ToString(getArg(args, 2))
		iniFile := advplrt.ToString(getArg(args, 3))

		val, err := iniGetKey(iniFile, section, key)
		if err != nil || val == "" {
			return advplrt.NewString(defaultVal), nil
		}
		return advplrt.NewString(val), nil
	}

	// GetSrvProfString(cChave, cDefault) -> cRet
	// Reads from the server's ENVIRONMENT section in appserver.ini (or configured via ADVPP_APPSERVER_INI)
	natives["GETSRVPROFSTRING"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := advplrt.ToString(getArg(args, 0))
		defaultVal := advplrt.ToString(getArg(args, 1))

		// Try to get from environment variable, then fall back to standard locations
		srvIniPath := os.Getenv("ADVPP_APPSERVER_INI")
		if srvIniPath == "" {
			// Look for appserver.ini in the current directory or standard locations
			// For AdvPP, we use a default or empty INI
			srvIniPath = "appserver.ini"
		}

		val, err := iniGetKey(srvIniPath, "ENVIRONMENT", key)
		if err != nil || val == "" {
			// Also try without section if "ENVIRONMENT" section doesn't exist
			val, err = iniGetKey(srvIniPath, "", key)
			if err != nil || val == "" {
				return advplrt.NewString(defaultVal), nil
			}
		}
		return advplrt.NewString(val), nil
	}

	// WriteSrvProfString(cChave, cValor) -> lRet
	// Writes to the server's ENVIRONMENT section in appserver.ini
	natives["WRITESRVPROFSTRING"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := advplrt.ToString(getArg(args, 0))
		value := advplrt.ToString(getArg(args, 1))

		srvIniPath := os.Getenv("ADVPP_APPSERVER_INI")
		if srvIniPath == "" {
			srvIniPath = "appserver.ini"
		}

		// Create the INI file if it doesn't exist
		if _, err := os.Stat(srvIniPath); err != nil && os.IsNotExist(err) {
			// Create empty INI with ENVIRONMENT section
			content := "[ENVIRONMENT]\n" + key + "=" + value + "\n"
			if err := os.WriteFile(srvIniPath, []byte(content), 0644); err != nil {
				return advplrt.NewBool(false), nil
			}
			return advplrt.NewBool(true), nil
		}

		success, err := iniSetKey(srvIniPath, "ENVIRONMENT", key, value)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(success), nil
	}
}

// toNumber converts a Value to float64, returning 0 if not numeric
func toNumber(val advplrt.Value) float64 {
	if val == nil || val == advplrt.Nil {
		return 0
	}
	if numVal, ok := val.(*advplrt.NumberValue); ok {
		return numVal.Val
	}
	// Try to parse as string
	s := advplrt.ToString(val)
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return n
}

// INI file manipulation functions

// iniLine represents a single line in an INI file (either a key=value, comment, or blank)
type iniLine struct {
	LineType string // "key", "comment", "blank"
	Key      string // only for "key" type (uppercase)
	Value    string // only for "key" type
	RawLine  string // original line for comments/blanks
}

type iniSection struct {
	Name   string
	Lines  []iniLine       // preserves order, comments, blanks
	Keys   []string        // insertion order of keys (for deterministic iteration)
	KeyMap map[string]int  // maps uppercase key to index in Keys array
}

type iniFile struct {
	Preamble []iniLine     // comments/blanks before first section
	Sections []iniSection
}

// parseINI parses an INI file and returns sections, preserving comments and blank lines
func parseINI(path string) ([]iniSection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file iniFile
	var currentSection *iniSection
	lines := strings.Split(string(data), "\n")

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// Handle blank lines
		if trimmed == "" {
			if currentSection != nil {
				currentSection.Lines = append(currentSection.Lines, iniLine{
					LineType: "blank",
					RawLine:  rawLine,
				})
			} else {
				// Blank line before first section: add to preamble
				file.Preamble = append(file.Preamble, iniLine{
					LineType: "blank",
					RawLine:  rawLine,
				})
			}
			continue
		}

		// Handle comment lines
		if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			if currentSection != nil {
				currentSection.Lines = append(currentSection.Lines, iniLine{
					LineType: "comment",
					RawLine:  rawLine,
				})
			} else {
				// Comment before first section: add to preamble
				file.Preamble = append(file.Preamble, iniLine{
					LineType: "comment",
					RawLine:  rawLine,
				})
			}
			continue
		}

		// Check for section header
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			newSection := iniSection{
				Name:   sectionName,
				Lines:  []iniLine{},
				Keys:   []string{},
				KeyMap: make(map[string]int),
			}
			file.Sections = append(file.Sections, newSection)
			currentSection = &file.Sections[len(file.Sections)-1]
			continue
		}

		// Parse key=value
		if currentSection != nil && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				upperKey := strings.ToUpper(key)

				// Track insertion order only once per key
				if _, exists := currentSection.KeyMap[upperKey]; !exists {
					currentSection.Keys = append(currentSection.Keys, upperKey)
					currentSection.KeyMap[upperKey] = len(currentSection.Keys) - 1
				}

				currentSection.Lines = append(currentSection.Lines, iniLine{
					LineType: "key",
					Key:      upperKey,
					Value:    value,
					RawLine:  rawLine,
				})
			}
		}
	}

	// Return just sections for backwards compatibility (parseINI returns []iniSection)
	// We need to store the preamble somewhere accessible, so we'll use a hack:
	// If there are pre-section comments/blanks, create a special "preamble" section with negative index
	if len(file.Preamble) > 0 {
		// Create a special section to hold preamble
		preambleSection := iniSection{
			Name:   "\x00PREAMBLE", // Special marker
			Lines:  file.Preamble,
			Keys:   []string{},
			KeyMap: make(map[string]int),
		}
		// Insert preamble as first section
		file.Sections = append([]iniSection{preambleSection}, file.Sections...)
	}

	return file.Sections, nil
}

// iniGetKey retrieves a value from an INI file
func iniGetKey(path, section, key string) (string, error) {
	sections, err := parseINI(path)
	if err != nil {
		return "", err
	}

	upperSection := strings.ToUpper(section)
	upperKey := strings.ToUpper(key)

	for _, sec := range sections {
		if strings.ToUpper(sec.Name) == upperSection {
			for _, line := range sec.Lines {
				if line.LineType == "key" && line.Key == upperKey {
					return line.Value, nil
				}
			}
			return "", nil
		}
	}

	// If section not found, return empty
	return "", nil
}

// iniSetKey sets a value in an INI file (create or update, preserving comments and formatting)
func iniSetKey(path, section, key, value string) (bool, error) {
	sections, err := parseINI(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	upperKey := strings.ToUpper(key)
	upperSection := strings.ToUpper(section)

	// Find or create section
	sectionIdx := -1
	for i, sec := range sections {
		if strings.ToUpper(sec.Name) == upperSection {
			sectionIdx = i
			break
		}
	}

	if sectionIdx == -1 {
		// Create new section
		newSection := iniSection{
			Name:   section,
			Lines:  []iniLine{{LineType: "key", Key: upperKey, Value: value}},
			Keys:   []string{upperKey},
			KeyMap: map[string]int{upperKey: 0},
		}
		sections = append(sections, newSection)
	} else {
		// Update existing section: find and replace key line, or append if not found
		keyFound := false
		for i, line := range sections[sectionIdx].Lines {
			if line.LineType == "key" && line.Key == upperKey {
				sections[sectionIdx].Lines[i].Value = value
				keyFound = true
				break
			}
		}

		if !keyFound {
			// Add new key to section
			if _, exists := sections[sectionIdx].KeyMap[upperKey]; !exists {
				sections[sectionIdx].Keys = append(sections[sectionIdx].Keys, upperKey)
				sections[sectionIdx].KeyMap[upperKey] = len(sections[sectionIdx].Keys) - 1
			}
			sections[sectionIdx].Lines = append(sections[sectionIdx].Lines, iniLine{
				LineType: "key",
				Key:      upperKey,
				Value:    value,
			})
		}
	}

	// Reconstruct file preserving comments and blank lines
	if !serializeINI(path, sections) {
		return false, nil
	}

	return true, nil
}

// iniDeleteKey deletes a key from a section, preserving comments and other formatting
func iniDeleteKey(path, section, key string) (bool, error) {
	sections, err := parseINI(path)
	if err != nil {
		return false, err
	}

	upperSection := strings.ToUpper(section)
	upperKey := strings.ToUpper(key)
	keyFound := false

	for i, sec := range sections {
		if strings.ToUpper(sec.Name) == upperSection {
			// Find and remove the key line
			for j := 0; j < len(sections[i].Lines); j++ {
				if sections[i].Lines[j].LineType == "key" && sections[i].Lines[j].Key == upperKey {
					sections[i].Lines = append(sections[i].Lines[:j], sections[i].Lines[j+1:]...)
					keyFound = true
					break
				}
			}
			// Also remove from Keys array to maintain consistency
			if keyFound {
				for j, k := range sections[i].Keys {
					if k == upperKey {
						sections[i].Keys = append(sections[i].Keys[:j], sections[i].Keys[j+1:]...)
						delete(sections[i].KeyMap, upperKey)
						break
					}
				}
			}
			break
		}
	}

	if !keyFound {
		return false, nil
	}

	if !serializeINI(path, sections) {
		return false, nil
	}

	return true, nil
}

// iniDeleteSection deletes a section from the INI file, preserving other sections' comments
func iniDeleteSection(path, section string) (bool, error) {
	sections, err := parseINI(path)
	if err != nil {
		return false, err
	}

	upperSection := strings.ToUpper(section)
	sectionFound := false
	var newSections []iniSection

	for _, sec := range sections {
		if strings.ToUpper(sec.Name) != upperSection {
			newSections = append(newSections, sec)
		} else {
			sectionFound = true
		}
	}

	if !sectionFound {
		return false, nil
	}

	if !serializeINI(path, newSections) {
		return false, nil
	}

	return true, nil
}

// serializeINI writes sections back to file in deterministic order (insertion-order for keys)
func serializeINI(path string, sections []iniSection) bool {
	var content strings.Builder

	for _, sec := range sections {
		// Skip preamble section header, but output its lines
		if sec.Name == "\x00PREAMBLE" {
			for _, line := range sec.Lines {
				if line.LineType == "comment" {
					content.WriteString(line.RawLine + "\n")
				} else if line.LineType == "blank" {
					content.WriteString(line.RawLine + "\n")
				}
			}
			continue
		}

		// Output section header (but not for preamble)
		content.WriteString("[" + sec.Name + "]\n")

		// Output lines in their original order (preserving comments/blanks)
		for _, line := range sec.Lines {
			if line.LineType == "key" {
				content.WriteString(line.Key + "=" + line.Value + "\n")
			} else if line.LineType == "comment" {
				content.WriteString(line.RawLine + "\n")
			} else if line.LineType == "blank" {
				content.WriteString(line.RawLine + "\n")
			}
		}
	}

	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		return false
	}

	return true
}

// iniGetSections retrieves all section names from an INI file (excluding internal preamble marker)
func iniGetSections(path string) ([]string, error) {
	sections, err := parseINI(path)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, sec := range sections {
		// Skip internal preamble marker (used to preserve comments before first section)
		if sec.Name != "\x00PREAMBLE" {
			names = append(names, sec.Name)
		}
	}

	return names, nil
}
