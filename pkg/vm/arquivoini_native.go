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

type iniSection struct {
	Name string
	Keys map[string]string
}

// parseINI parses an INI file and returns sections
func parseINI(path string) ([]iniSection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sections []iniSection
	var currentSection *iniSection
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.TrimSpace(line[1 : len(line)-1])
			currentSection = &iniSection{
				Name: sectionName,
				Keys: make(map[string]string),
			}
			sections = append(sections, *currentSection)
			continue
		}

		// Parse key=value
		if currentSection != nil && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				currentSection.Keys[strings.ToUpper(key)] = value
			}
		}
	}

	return sections, nil
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
			if val, ok := sec.Keys[upperKey]; ok {
				return val, nil
			}
			return "", nil
		}
	}

	// If section not found, return empty
	return "", nil
}

// iniSetKey sets a value in an INI file (create or update)
func iniSetKey(path, section, key, value string) (bool, error) {
	sections, err := parseINI(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	// Ensure section exists
	sectionFound := false
	for i, sec := range sections {
		if strings.ToUpper(sec.Name) == strings.ToUpper(section) {
			sections[i].Keys[strings.ToUpper(key)] = value
			sectionFound = true
			break
		}
	}

	if !sectionFound {
		sections = append(sections, iniSection{
			Name: section,
			Keys: map[string]string{strings.ToUpper(key): value},
		})
	}

	// Write back to file
	var content strings.Builder
	for _, sec := range sections {
		content.WriteString("[" + sec.Name + "]\n")
		for k, v := range sec.Keys {
			content.WriteString(k + "=" + v + "\n")
		}
	}

	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// iniDeleteKey deletes a key from a section
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
			if _, ok := sections[i].Keys[upperKey]; ok {
				delete(sections[i].Keys, upperKey)
				keyFound = true
			}
			break
		}
	}

	if !keyFound {
		return false, nil
	}

	// Write back to file
	var content strings.Builder
	for _, sec := range sections {
		content.WriteString("[" + sec.Name + "]\n")
		for k, v := range sec.Keys {
			content.WriteString(k + "=" + v + "\n")
		}
	}

	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// iniDeleteSection deletes a section from the INI file
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

	// Write back to file
	var content strings.Builder
	for _, sec := range newSections {
		content.WriteString("[" + sec.Name + "]\n")
		for k, v := range sec.Keys {
			content.WriteString(k + "=" + v + "\n")
		}
	}

	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// iniGetSections retrieves all section names from an INI file
func iniGetSections(path string) ([]string, error) {
	sections, err := parseINI(path)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, sec := range sections {
		names = append(names, sec.Name)
	}

	return names, nil
}
