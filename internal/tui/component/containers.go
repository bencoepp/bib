package component

import (
	"strings"

	"bib/internal/tui/themes"

	"github.com/charmbracelet/lipgloss"
)

// Card is a bordered content container
type Card struct {
	BaseComponent

	title       string
	content     string
	footer      string
	bordered    bool
	shadow      bool
	padding     int
	borderStyle lipgloss.Border
}

// NewCard creates a new card
func NewCard() *Card {
	return &Card{
		BaseComponent: NewBaseComponent(),
		bordered:      true,
		padding:       1,
		borderStyle:   lipgloss.RoundedBorder(),
	}
}

// WithTitle sets the card title
func (c *Card) WithTitle(title string) *Card {
	c.title = title
	return c
}

// WithContent sets the card content
func (c *Card) WithContent(content string) *Card {
	c.content = content
	return c
}

// WithFooter sets the card footer
func (c *Card) WithFooter(footer string) *Card {
	c.footer = footer
	return c
}

// WithBorder enables/disables the border
func (c *Card) WithBorder(bordered bool) *Card {
	c.bordered = bordered
	return c
}

// WithShadow enables/disables shadow effect
func (c *Card) WithShadow(shadow bool) *Card {
	c.shadow = shadow
	return c
}

// WithPadding sets the padding
func (c *Card) WithPadding(padding int) *Card {
	c.padding = padding
	return c
}

// WithBorderStyle sets the border style
func (c *Card) WithBorderStyle(style lipgloss.Border) *Card {
	c.borderStyle = style
	return c
}

// WithTheme sets the theme
func (c *Card) WithTheme(theme *themes.Theme) *Card {
	c.SetTheme(theme)
	return c
}

// View implements Component
func (c *Card) View(width int) string {
	theme := c.Theme()
	var content strings.Builder

	// Title
	if c.title != "" {
		content.WriteString(theme.CardTitle.Render(c.title))
		content.WriteString("\n")
		if c.content != "" || c.footer != "" {
			content.WriteString("\n")
		}
	}

	// Content
	if c.content != "" {
		content.WriteString(c.content)
		if c.footer != "" {
			content.WriteString("\n\n")
		}
	}

	// Footer
	if c.footer != "" {
		content.WriteString(theme.Blurred.Render(c.footer))
	}

	style := theme.Card
	if c.bordered {
		style = style.Border(c.borderStyle).
			BorderForeground(theme.Palette.Border)
	}
	if c.padding > 0 {
		style = style.Padding(c.padding-1, c.padding)
	}
	if width > 0 {
		style = style.Width(width)
	}

	result := style.Render(content.String())

	// Simple shadow effect using a shifted darker box
	if c.shadow {
		lines := strings.Split(result, "\n")
		for i := range lines {
			lines[i] = lines[i] + theme.Blurred.Render("░")
		}
		shadowLine := strings.Repeat("░", lipgloss.Width(result)+1)
		result = strings.Join(lines, "\n") + "\n" + theme.Blurred.Render(shadowLine)
	}

	return result
}

// Box is a simpler bordered container
type Box struct {
	BaseComponent

	content     string
	title       string
	bordered    bool
	borderStyle lipgloss.Border
	padding     int
}

// NewBox creates a new box
func NewBox(content string) *Box {
	return &Box{
		BaseComponent: NewBaseComponent(),
		content:       content,
		bordered:      true,
		borderStyle:   lipgloss.RoundedBorder(),
		padding:       1,
	}
}

// WithTitle sets the box title
func (b *Box) WithTitle(title string) *Box {
	b.title = title
	return b
}

// WithBorder enables/disables the border
func (b *Box) WithBorder(bordered bool) *Box {
	b.bordered = bordered
	return b
}

// WithBorderStyle sets the border style
func (b *Box) WithBorderStyle(style lipgloss.Border) *Box {
	b.borderStyle = style
	return b
}

// WithPadding sets the padding
func (b *Box) WithPadding(padding int) *Box {
	b.padding = padding
	return b
}

// WithTheme sets the theme
func (b *Box) WithTheme(theme *themes.Theme) *Box {
	b.SetTheme(theme)
	return b
}

// View implements Component
func (b *Box) View(width int) string {
	theme := b.Theme()
	var content strings.Builder

	if b.title != "" {
		content.WriteString(theme.BoxTitle.Render(b.title))
		content.WriteString("\n\n")
	}
	content.WriteString(b.content)

	style := theme.Box
	if b.bordered {
		style = style.Border(b.borderStyle).
			BorderForeground(theme.Palette.Border)
	}
	if b.padding > 0 {
		style = style.Padding(b.padding-1, b.padding)
	}
	if width > 0 {
		style = style.Width(width)
	}

	return style.Render(content.String())
}

// Panel is a full-width section container
type Panel struct {
	BaseComponent

	title   string
	content string
	margin  int
}

// NewPanel creates a new panel
func NewPanel() *Panel {
	return &Panel{
		BaseComponent: NewBaseComponent(),
		margin:        1,
	}
}

// WithTitle sets the panel title
func (p *Panel) WithTitle(title string) *Panel {
	p.title = title
	return p
}

// WithContent sets the panel content
func (p *Panel) WithContent(content string) *Panel {
	p.content = content
	return p
}

// WithMargin sets the margin
func (p *Panel) WithMargin(margin int) *Panel {
	p.margin = margin
	return p
}

// WithTheme sets the theme
func (p *Panel) WithTheme(theme *themes.Theme) *Panel {
	p.SetTheme(theme)
	return p
}

// View implements Component
func (p *Panel) View(width int) string {
	theme := p.Theme()
	var b strings.Builder

	if p.title != "" {
		b.WriteString(theme.SectionTitle.Render(p.title))
		b.WriteString("\n")
	}

	b.WriteString(p.content)

	style := theme.Section
	if p.margin > 0 {
		style = style.MarginTop(p.margin).MarginBottom(p.margin)
	}
	if width > 0 {
		style = style.Width(width)
	}

	return style.Render(b.String())
}

// KeyValue renders a key-value pair
type KeyValue struct {
	BaseComponent

	key      string
	value    string
	keyWidth int
}

// NewKeyValue creates a new key-value pair
func NewKeyValue(key, value string) *KeyValue {
	return &KeyValue{
		BaseComponent: NewBaseComponent(),
		key:           key,
		value:         value,
		keyWidth:      20,
	}
}

// WithKeyWidth sets the key column width
func (kv *KeyValue) WithKeyWidth(width int) *KeyValue {
	kv.keyWidth = width
	return kv
}

// WithTheme sets the theme
func (kv *KeyValue) WithTheme(theme *themes.Theme) *KeyValue {
	kv.SetTheme(theme)
	return kv
}

// View implements Component
func (kv *KeyValue) View(width int) string {
	theme := kv.Theme()
	keyStyle := theme.Focused.Width(kv.keyWidth)
	return keyStyle.Render(kv.key+":") + " " + theme.Base.Render(kv.value)
}

// KeyValueList renders multiple key-value pairs
type KeyValueList struct {
	BaseComponent

	items    []struct{ key, value string }
	keyWidth int
}

// NewKeyValueList creates a new key-value list
func NewKeyValueList() *KeyValueList {
	return &KeyValueList{
		BaseComponent: NewBaseComponent(),
		keyWidth:      20,
	}
}

// Add adds a key-value pair
func (kvl *KeyValueList) Add(key, value string) *KeyValueList {
	kvl.items = append(kvl.items, struct{ key, value string }{key, value})
	return kvl
}

// WithKeyWidth sets the key column width
func (kvl *KeyValueList) WithKeyWidth(width int) *KeyValueList {
	kvl.keyWidth = width
	return kvl
}

// WithTheme sets the theme
func (kvl *KeyValueList) WithTheme(theme *themes.Theme) *KeyValueList {
	kvl.SetTheme(theme)
	return kvl
}

// View implements Component
func (kvl *KeyValueList) View(width int) string {
	var lines []string
	for _, item := range kvl.items {
		lines = append(lines, NewKeyValue(item.key, item.value).
			WithKeyWidth(kvl.keyWidth).
			WithTheme(kvl.Theme()).
			View(width))
	}
	return strings.Join(lines, "\n")
}

// Divider renders a horizontal divider
type Divider struct {
	BaseComponent

	style DividerStyle
}

// DividerStyle defines the divider appearance
type DividerStyle int

const (
	DividerSingle DividerStyle = iota
	DividerDouble
	DividerDashed
	DividerDotted
)

// NewDivider creates a new divider
func NewDivider() *Divider {
	return &Divider{
		BaseComponent: NewBaseComponent(),
		style:         DividerSingle,
	}
}

// WithStyle sets the divider style
func (d *Divider) WithStyle(style DividerStyle) *Divider {
	d.style = style
	return d
}

// WithTheme sets the theme
func (d *Divider) WithTheme(theme *themes.Theme) *Divider {
	d.SetTheme(theme)
	return d
}

// View implements Component
func (d *Divider) View(width int) string {
	if width <= 0 {
		width = 40
	}

	var char string
	switch d.style {
	case DividerSingle:
		char = "─"
	case DividerDouble:
		char = "═"
	case DividerDashed:
		char = "╌"
	case DividerDotted:
		char = "·"
	}

	return d.Theme().Divider.Render(strings.Repeat(char, width))
}
