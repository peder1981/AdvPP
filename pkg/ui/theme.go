package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// advppPrimary é o mesmo azul-petróleo usado no tema web (advplc serve,
// web/src/styles.css --color-brand-01-base) — consistência visual entre
// os dois jeitos de rodar um programa AdvPP.
var advppPrimary = color.NRGBA{R: 0x0f, G: 0x6e, B: 0x68, A: 0xff}

// Theme retema só a cor primária (botões, seleção, foco) sobre o tema
// padrão do Fyne — mantém contraste/acessibilidade do tema base intactos,
// só troca a matiz de roxo/azul padrão pela identidade visual do AdvPP.
type appTheme struct {
	fyne.Theme
}

// NewTheme retorna o tema visual padrão do AdvPP pra aplicações desktop
// (standalone build e advpp-ide) — usar em vez de theme.DefaultTheme().
func NewTheme() fyne.Theme {
	return &appTheme{Theme: theme.DefaultTheme()}
}

func (t *appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return advppPrimary
	}
	return t.Theme.Color(name, variant)
}
