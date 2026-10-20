package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

var (
	tuiAccentColor = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
	tuiStrongColor = lipgloss.AdaptiveColor{Light: "#4C1D95", Dark: "#A78BFA"}
	tuiMutedColor  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	tuiErrorColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}

	tuiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(tuiAccentColor)

	tuiSubtitleStyle = lipgloss.NewStyle().
				Foreground(tuiMutedColor)

	tuiLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(tuiStrongColor)

	tuiFocusedLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(tuiAccentColor)

	tuiStatusStyle = lipgloss.NewStyle().
			Foreground(tuiStrongColor)

	tuiStatusErrorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(tuiErrorColor)

	tuiSpinnerStyle = lipgloss.NewStyle().
			Foreground(tuiAccentColor)

	tuiCursorStyle = lipgloss.NewStyle().
			Foreground(tuiAccentColor)

	tuiTabActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#6D28D9")).
				Padding(0, 1)

	tuiTabInactiveStyle = lipgloss.NewStyle().
				Foreground(tuiMutedColor).
				Padding(0, 1)

	tuiModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tuiAccentColor).
			Padding(1, 2)

	tuiBodyStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

func tuiListStyles() list.DefaultItemStyles {
	styles := list.NewDefaultItemStyles()

	styles.NormalTitle = styles.NormalTitle.
		Foreground(tuiStrongColor)
	styles.NormalDesc = styles.NormalDesc.
		Foreground(tuiMutedColor)
	styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiAccentColor).
		Foreground(tuiAccentColor).
		Padding(0, 0, 0, 1)
	styles.SelectedDesc = styles.SelectedTitle.
		Bold(false).
		Foreground(tuiMutedColor)
	styles.DimmedTitle = styles.DimmedTitle.
		Foreground(tuiMutedColor)
	styles.DimmedDesc = styles.DimmedDesc.
		Foreground(tuiMutedColor)

	return styles
}

func (m tuiModel) View() string {
	switch m.overlay {
	case tuiOverlayHelp:
		return m.helpOverlayView()
	case tuiOverlayOutput:
		return m.outputOverlayView()
	case tuiOverlayConfirm:
		return m.confirmOverlayView()
	default:
		body := tuiBodyStyle.
			Height(maxInt(1, m.height-4)).
			Render(m.bodyView())

		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.headerView(),
			body,
			m.footerView(),
		)
	}
}

func (m tuiModel) headerView() string {
	path := "sem instância"
	if m.inst != nil {
		path = m.inst.Path
	}
	title := tuiTitleStyle.Render("Modink")
	pathView := ""
	if maxPath := m.width - 20; maxPath >= 8 {
		path = truncateMiddle(sanitizeText(path), maxPath)
		pathView = "  " + tuiSubtitleStyle.Render(path)
	}

	return " " + title + pathView + "\n" + m.tabsView()
}

func (m tuiModel) tabsView() string {
	search := tuiTabStyle("1", "Buscar", m.tab == tuiTabSearch)
	installed := tuiTabStyle("2", "Instalados", m.tab == tuiTabInstalled)

	return " " + search + " " + installed
}

func tuiTabStyle(key, label string, active bool) string {
	text := key + " " + label
	if active {
		return tuiTabActiveStyle.Render(text)
	}

	return tuiTabInactiveStyle.Render(text)
}

func (m tuiModel) bodyView() string {
	if m.tab == tuiTabInstalled {
		return m.installedView()
	}
	if m.project != nil {
		return m.projectView()
	}

	return m.searchView()
}

func (m tuiModel) searchView() string {
	rows := []string{
		m.textFieldView("Busca", m.query.View(), tuiFocusQuery),
		m.textFieldView("Versão", m.gameVersion.View(), tuiFocusGameVersion),
		m.textFieldView("Loader", m.loader.View(), tuiFocusLoader),
		m.selectorView("Tipo", tuiValueAt(tuiSearchKindLabels, m.typeIndex), tuiFocusType),
		m.selectorView("Canal", tuiValueAt(tuiChannelLabels, m.channelIndex), tuiFocusChannel),
	}

	header := "Resultados"
	if m.searchTotal > 0 {
		header = fmt.Sprintf("Resultados: %d", m.searchTotal)
	}

	return strings.Join(rows, "\n") +
		"\n" +
		tuiSubtitleStyle.Render(header) +
		"\n" +
		m.searchResultsView()
}

func (m tuiModel) textFieldView(label, input string, focus tuiFocus) string {
	marker := "  "
	labelStyle := tuiLabelStyle
	if m.focus == focus && !m.busy {
		marker = "▸ "
		labelStyle = tuiFocusedLabelStyle
	}

	return marker + labelStyle.Render(padRight(label, 10)) + input
}

func (m tuiModel) selectorView(label, value string, focus tuiFocus) string {
	marker := "  "
	labelStyle := tuiLabelStyle
	valueStyle := tuiSubtitleStyle

	if m.focus == focus && !m.busy {
		marker = "▸ "
		labelStyle = tuiFocusedLabelStyle
		valueStyle = tuiStatusStyle
		value = "◂ " + value + " ▸"
	}

	return marker + labelStyle.Render(padRight(label, 10)) + valueStyle.Render(value)
}

func (m tuiModel) searchResultsView() string {
	if len(m.searchProjects) > 0 {
		return m.searchList.View()
	}

	message := "Digite um termo e pressione Enter para buscar."
	switch {
	case m.busy:
		message = "Buscando projetos..."
	case m.searchDone:
		message = "Nenhum projeto encontrado com esses filtros."
	case m.searchFormHasInput():
		message = "Pressione Enter para buscar."
	}

	return tuiSubtitleStyle.Render(truncate(message, maxInt(10, m.width-4)))
}

func (m tuiModel) installedView() string {
	tracked := 0
	for _, item := range m.installed {
		if item.tracked {
			tracked++
		}
	}
	untracked := len(m.installed) - tracked

	overwrite := "desligada"
	if m.overwrite {
		overwrite = "ligada"
	}

	summary := truncate(fmt.Sprintf(
		"%d rastreado(s) • %d não rastreado(s) • sobrescrita: %s • deps: %s",
		tracked,
		untracked,
		overwrite,
		tuiYesNo(m.installDependencies),
	), maxInt(10, m.width-4))

	if len(m.installed) == 0 {
		message := truncate("Nenhum conteúdo encontrado nas pastas gerenciadas.", maxInt(10, m.width-4))
		return tuiSubtitleStyle.Render(summary) +
			"\n\n" +
			tuiSubtitleStyle.Render(message)
	}

	return tuiSubtitleStyle.Render(summary) + "\n\n" + m.installedList.View()
}

func (m tuiModel) projectView() string {
	project := *m.project

	title := tuiTitleStyle.Render(truncate(projectDisplayName(project), maxInt(8, m.width-24)))
	if kind := displayProjectType(project.Type); kind != "-" {
		title += tuiSubtitleStyle.Render("  " + truncate(kind, 20))
	}

	summary := wrapText(project.Summary, maxInt(20, m.width-6), 2)
	if strings.TrimSpace(summary) == "" {
		summary = tuiSubtitleStyle.Render("Sem resumo.")
	}

	channels := tuiChannelsLabel(m.projectFilter.Channels)
	filters := truncate(fmt.Sprintf(
		"Filtros: jogo=%s • loader=%s • canal=%s",
		tuiValueOrDash(m.projectFilter.GameVersion),
		tuiValueOrDash(m.projectFilter.Loader),
		channels,
	), maxInt(10, m.width-4))

	toggles := truncate(fmt.Sprintf(
		"Sobrescrita: %s (o) • Dependências: %s (d)",
		tuiYesNo(m.overwrite),
		tuiYesNo(m.installDependencies),
	), maxInt(10, m.width-4))

	meta := "Autores: " + authorsText(project.Authors)
	if project.Downloads > 0 {
		meta += " • " + formatCount(project.Downloads) + " downloads"
	}
	lines := []string{
		title,
		tuiSubtitleStyle.Render(truncate(meta, maxInt(10, m.width-4))),
	}
	lines = append(lines, strings.Split(summary, "\n")...)
	lines = append(lines, tuiSubtitleStyle.Render(filters))
	lines = append(lines, tuiSubtitleStyle.Render(toggles))

	if _, err := installDirectoryName(project.Type); err != nil {
		lines = append(lines, tuiStatusErrorStyle.Render(truncate("Instalação não suportada: "+err.Error(), maxInt(10, m.width-4))))
	}

	lines = append(lines, "")
	lines = append(lines, tuiSubtitleStyle.Render(truncate(
		fmt.Sprintf("Versões compatíveis: %d", len(m.versions)),
		maxInt(10, m.width-4),
	)))

	if len(m.versions) == 0 {
		lines = append(lines, tuiSubtitleStyle.Render(truncate("Nenhuma versão encontrada com esses filtros.", maxInt(10, m.width-4))))
		return strings.Join(lines, "\n")
	}

	return strings.Join(lines, "\n") + "\n" + m.versionList.View()
}

func (m tuiModel) footerView() string {
	return m.statusLine() + "\n" + tuiSubtitleStyle.Render(truncate(m.keyHints(), maxInt(10, m.width-2)))
}

func (m tuiModel) statusLine() string {
	if m.busy {
		return " " + m.spinner.View() + " " + tuiSubtitleStyle.Render(truncate(m.busyLabel, maxInt(10, m.width-6)))
	}
	if strings.TrimSpace(m.status) == "" {
		message := truncate("Use F1/F2 para alternar as telas e ? para ver a ajuda.", maxInt(10, m.width-4))
		return " " + tuiSubtitleStyle.Render(message)
	}

	status := truncate(m.status, maxInt(10, m.width-2))
	if m.statusError {
		return " " + tuiStatusErrorStyle.Render(status)
	}

	return " " + tuiStatusStyle.Render(status)
}

func (m tuiModel) keyHints() string {
	if m.busy {
		return "aguarde... • ctrl+c sair"
	}

	if m.tab == tuiTabInstalled {
		return "↑/↓ mover • enter detalhes • u atualizar • U todos • c verificar • C todos • x remover • r recarregar • ? ajuda • q sair"
	}

	if m.project != nil {
		return "↑/↓ versões • i/enter instalar • o sobrescrever • d dependências • r recarregar • esc voltar • ? ajuda • q sair"
	}

	if m.focus == tuiFocusResults {
		return "↑/↓ resultados • enter projeto • / buscar • r repetir • f1/f2 telas • ? ajuda • q sair"
	}

	return "tab campos • enter buscar • esc resultados • f1/f2 telas • ? ajuda • ctrl+c sair"
}

func (m tuiModel) helpOverlayView() string {
	width := minInt(78, maxInt(30, m.width-8))
	height := maxInt(8, m.height-8)

	title := tuiTitleStyle.Render("Ajuda")
	hint := tuiSubtitleStyle.Render("esc/?/q fecha • ↑/↓ rola")

	content := title + "\n\n" + m.helpView.View() + "\n\n" + hint
	box := tuiModalStyle.Width(width).Height(height).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func tuiHelpContent(width int) string {
	lines := []struct {
		text  string
		style lipgloss.Style
	}{
		{text: "Navegação: Tab/Shift+Tab campos • F1/F2 (ou 1/2 fora de campos) alterna telas • ↑/↓ listas", style: tuiLabelStyle},
		{text: "Esc volta ou fecha • q sai • Ctrl+C sai imediatamente"},
		{},
		{text: "Busca: Enter busca • / foca a busca • ←/→ tipo/canal", style: tuiLabelStyle},
		{text: "Projeto: i/Enter instala • o sobrescreve • d dependências • r recarrega", style: tuiLabelStyle},
		{text: "Instalados: u atualiza • U todos • c verifica • C todos", style: tuiLabelStyle},
		{text: "x/Delete remove • Enter detalhes • o sobrescreve • d dependências"},
		{text: "r recarrega o manifesto • v abre a última saída"},
	}

	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			rendered = append(rendered, "")
			continue
		}

		rendered = append(rendered, line.style.Render(wrapText(line.text, width, 0)))
	}

	return strings.Join(rendered, "\n")
}

func (m tuiModel) outputOverlayView() string {
	width := maxInt(24, m.width-8)
	height := maxInt(6, m.height-8)

	title := tuiTitleStyle.Render(m.lastOutputTitle)
	hint := tuiSubtitleStyle.Render("esc/enter fecha • ↑/↓ rola • o sobrescrever")

	content := title + "\n\n" + m.outputView.View() + "\n\n" + hint
	box := tuiModalStyle.Width(width).Height(height).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m tuiModel) confirmOverlayView() string {
	width := minInt(70, maxInt(30, m.width-8))
	contentWidth := maxInt(10, width-4)

	body := wrapText(m.confirm.body, contentWidth, 8)
	actions := tuiLabelStyle.Render("s/Enter") + " confirmar    " +
		tuiLabelStyle.Render("n/Esc") + " cancelar"

	content := tuiTitleStyle.Render(m.confirm.title) +
		"\n\n" +
		body +
		"\n\n" +
		actions

	box := tuiModalStyle.Width(width).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func tuiValueAt(values []string, index int) string {
	if len(values) == 0 {
		return "-"
	}

	return values[wrapIndex(index, len(values))]
}

func tuiChannelsLabel(channels []modsource.ReleaseChannel) string {
	if len(channels) == 0 {
		return "todos"
	}

	values := make([]string, 0, len(channels))
	for _, channel := range channels {
		values = append(values, displayChannel(channel))
	}

	return strings.Join(values, ",")
}

func tuiValueOrDash(value string) string {
	value = strings.TrimSpace(sanitizeText(value))
	if value == "" {
		return "-"
	}

	return value
}

func tuiYesNo(value bool) string {
	if value {
		return "sim"
	}

	return "não"
}

func padRight(value string, width int) string {
	runes := []rune(sanitizeText(value))
	if len(runes) >= width {
		return string(runes[:width])
	}

	return string(runes) + strings.Repeat(" ", width-len(runes))
}

func truncateMiddle(value string, max int) string {
	runes := []rune(sanitizeText(value))
	if max <= 0 {
		return ""
	}
	if len(runes) <= max {
		return string(runes)
	}
	if max <= 3 {
		return string(runes[:max])
	}

	left := (max - 1) / 2
	right := max - 1 - left

	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func wrapText(value string, width, maxLines int) string {
	value = strings.TrimSpace(sanitizeBlock(value))
	if value == "" || width <= 0 {
		return ""
	}

	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		line := ""
		for _, word := range words {
			wordRunes := []rune(word)
			if len(wordRunes) > width {
				word = string(wordRunes[:width])
			}

			switch {
			case line == "":
				line = word
			case len([]rune(line))+1+len([]rune(word)) > width:
				lines = append(lines, line)
				line = word
				if maxLines > 0 && len(lines) >= maxLines {
					break
				}
			default:
				line += " " + word
			}

			if maxLines > 0 && len(lines) >= maxLines {
				break
			}
		}

		if maxLines > 0 && len(lines) >= maxLines {
			break
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return strings.Join(lines, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
