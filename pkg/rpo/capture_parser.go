package rpo

import (
	"encoding/json"
	"fmt"
	"os"
)

// CaptureParser parseia arquivo de captura e identifica ciphers
type CaptureParser struct {
	events    []CaptureEvent
	segments  []CaptureSegment
	heuristic *CipherHeuristic
}

// NewCaptureParser cria novo parser
func NewCaptureParser() *CaptureParser {
	return &CaptureParser{
		heuristic: NewCipherHeuristic(),
	}
}

// LoadFile carrega eventos de arquivo JSON
func (p *CaptureParser) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("lendo captura: %w", err)
	}
	
	return p.LoadData(data)
}

// LoadData carrega eventos de bytes JSON
func (p *CaptureParser) LoadData(data []byte) error {
	var events []CaptureEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("parseando JSON: %w", err)
	}
	
	p.events = events
	p.segments = MergeCaptureSegments(events)
	return nil
}

// IdentifyCiphers tenta identificar ciphers usando heurística
func (p *CaptureParser) IdentifyCiphers() error {
	for i := range p.segments {
		seg := &p.segments[i]
		
		// Se já tem cipher name, pular
		if seg.Cipher != "" {
			continue
		}
		
		// Tentar identificar por heurística
		cipherInfo, err := p.heuristic.Identify(seg.Key, seg.IV)
		if err != nil {
			continue
		}
		
		seg.Cipher = cipherInfo.Name
	}
	
	return nil
}

// GetSegments retorna segmentos identificados
func (p *CaptureParser) GetSegments() []CaptureSegment {
	return p.segments
}

// PrintSummary imprime resumo da captura
func (p *CaptureParser) PrintSummary() {
	fmt.Printf("Total de eventos: %d\n", len(p.events))
	
	// Contar tipos
	types := make(map[string]int)
	for _, e := range p.events {
		types[e.Type]++
	}
	
	fmt.Println("\nTipos de eventos:")
	for t, count := range types {
		fmt.Printf("  %s: %d\n", t, count)
	}
	
	// Contar ciphers identificados
	ciphers := make(map[string]int)
	for _, s := range p.segments {
		if s.Cipher != "" {
			ciphers[s.Cipher]++
		}
	}
	
	if len(ciphers) > 0 {
		fmt.Println("\nCiphers identificados:")
		for c, count := range ciphers {
			fmt.Printf("  %s: %d\n", c, count)
		}
	}
}
