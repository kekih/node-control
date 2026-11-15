package native

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ThemeName string

const (
	ThemeDark   ThemeName = "dark"
	ThemeLight  ThemeName = "light"
	ThemeGreen  ThemeName = "green"
	ThemePurple ThemeName = "purple"
)

type ColorPalette struct {
	Name             ThemeName
	Title            string
	Background       tcell.Color
	CardBg           tcell.Color
	Border           tcell.Color
	Accent           tcell.Color
	TextMain         tcell.Color
	TextMuted        tcell.Color
	Success          tcell.Color
	Danger           tcell.Color
	Warning          tcell.Color
}

var Palettes = map[ThemeName]ColorPalette{
	ThemeDark: {
		Name:       ThemeDark,
		Title:      "🌙 Тёмная (Dark)",
		Background: tcell.NewHexColor(0x080d1a),
		CardBg:     tcell.NewHexColor(0x111c33),
		Border:     tcell.NewHexColor(0x1e2e4f),
		Accent:     tcell.NewHexColor(0x00c2ff),
		TextMain:   tcell.NewHexColor(0xffffff),
		TextMuted:  tcell.NewHexColor(0x8fa0b8),
		Success:    tcell.NewHexColor(0x10b981),
		Danger:     tcell.NewHexColor(0xef4444),
		Warning:    tcell.NewHexColor(0xf59e0b),
	},
	ThemeLight: {
		Name:       ThemeLight,
		Title:      "☀️ Светлая (Light)",
		Background: tcell.NewHexColor(0xf1f5f9),
		CardBg:     tcell.NewHexColor(0xffffff),
		Border:     tcell.NewHexColor(0xcbd5e1),
		Accent:     tcell.NewHexColor(0x0284c7),
		TextMain:   tcell.NewHexColor(0x0f172a),
		TextMuted:  tcell.NewHexColor(0x64748b),
		Success:    tcell.NewHexColor(0x059669),
		Danger:     tcell.NewHexColor(0xdc2626),
		Warning:    tcell.NewHexColor(0xd97706),
	},
	ThemeGreen: {
		Name:       ThemeGreen,
		Title:      "🌿 Зелёная (Matrix Emerald)",
		Background: tcell.NewHexColor(0x05140d),
		CardBg:     tcell.NewHexColor(0x0b2518),
		Border:     tcell.NewHexColor(0x154c33),
		Accent:     tcell.NewHexColor(0x10b981),
		TextMain:   tcell.NewHexColor(0xecfdf5),
		TextMuted:  tcell.NewHexColor(0x6ee7b7),
		Success:    tcell.NewHexColor(0x34d399),
		Danger:     tcell.NewHexColor(0xf87171),
		Warning:    tcell.NewHexColor(0xfbbf24),
	},
	ThemePurple: {
		Name:       ThemePurple,
		Title:      "🔮 Фиолетовая (Cyber Violet)",
		Background: tcell.NewHexColor(0x0e0819),
		CardBg:     tcell.NewHexColor(0x1b1130),
		Border:     tcell.NewHexColor(0x392263),
		Accent:     tcell.NewHexColor(0xa855f7),
		TextMain:   tcell.NewHexColor(0xfaf5ff),
		TextMuted:  tcell.NewHexColor(0xc084fc),
		Success:    tcell.NewHexColor(0x34d399),
		Danger:     tcell.NewHexColor(0xf43f5e),
		Warning:    tcell.NewHexColor(0xfbbf24),
	},
}

func ApplyTheme(name ThemeName) ColorPalette {
	p, ok := Palettes[name]
	if !ok {
		p = Palettes[ThemeDark]
	}

	tview.Styles.PrimitiveBackgroundColor = p.Background
	tview.Styles.ContrastBackgroundColor = p.CardBg
	tview.Styles.MoreContrastBackgroundColor = p.Accent
	tview.Styles.BorderColor = p.Border
	tview.Styles.TitleColor = p.Accent
	tview.Styles.GraphicsColor = p.Accent
	tview.Styles.PrimaryTextColor = p.TextMain
	tview.Styles.SecondaryTextColor = p.TextMuted
	tview.Styles.TertiaryTextColor = p.Border
	tview.Styles.InverseTextColor = p.Background
	tview.Styles.ContrastSecondaryTextColor = p.Accent

	return p
}
