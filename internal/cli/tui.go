package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/wedemmmdev/mcjava-manager/internal/download"
	"github.com/wedemmmdev/mcjava-manager/internal/instance"
	"github.com/wedemmmdev/mcjava-manager/internal/manifest"
	"github.com/wedemmmdev/mcjava-manager/pkg/modsource"
)

const (
	tuiSearchLimit   = 50
	tuiDefaultWidth  = 80
	tuiDefaultHeight = 24
)

type tuiTab int

const (
	tuiTabSearch tuiTab = iota
	tuiTabInstalled
)

type tuiFocus int

const (
	tuiFocusQuery tuiFocus = iota
	tuiFocusGameVersion
	tuiFocusLoader
	tuiFocusType
	tuiFocusChannel
	tuiFocusResults
)

type tuiOverlay int

const (
	tuiOverlayNone tuiOverlay = iota
	tuiOverlayHelp
	tuiOverlayOutput
	tuiOverlayConfirm
)

type tuiConfirmKind int

const (
	tuiConfirmRemove tuiConfirmKind = iota
)

type tuiConfirm struct {
	kind  tuiConfirmKind
	title string
	body  string
	query string
	entry modsource.ProjectKind
}

type tuiModel struct {
	ctx        context.Context
	client     modsource.Client
	downloader *download.Client
	inst       *instance.Instance
	store      *manifest.Store

	width  int
	height int

	tab     tuiTab
	focus   tuiFocus
	overlay tuiOverlay

	busy      bool
	busyLabel string
	spinner   spinner.Model
	seq       int

	status      string
	statusError bool

	lastOutput      string
	lastOutputTitle string
	outputView      viewport.Model
	helpView        viewport.Model

	query        textinput.Model
	gameVersion  textinput.Model
	loader       textinput.Model
	typeIndex    int
	channelIndex int

	searchList    list.Model
	installedList list.Model
	versionList   list.Model

	searchProjects  []modsource.Project
	lastSearchQuery modsource.SearchQuery
	searchTotal     int
	searchDone      bool

	project                *modsource.Project
	versions               []modsource.Version
	projectFilter          modsource.VersionFilter
	projectChannelExplicit bool

	overwrite           bool
	installDependencies bool

	installed []tuiInstalledItem
	confirm   tuiConfirm
}

type tuiSearchMsg struct {
	seq    int
	query  modsource.SearchQuery
	result modsource.SearchResult
	err    error
}

type tuiProjectMsg struct {
	seq             int
	project         modsource.Project
	versions        []modsource.Version
	filter          modsource.VersionFilter
	channelExplicit bool
	err             error
}

type tuiInstallMsg struct {
	seq         int
	projectName string
	outcomes    []installOutcome
	output      string
	err         error
}

type tuiUpdateMsg struct {
	seq    int
	check  bool
	all    bool
	output string
	err    error
}

type tuiRemoveMsg struct {
	seq    int
	output string
	err    error
}

// runTUI opens the Bubble Tea interface for an instance.
func (a *App) runTUI(ctx context.Context, opts *options) error {
	inst, err := instance.Prepare(opts.InstancePath)
	if err != nil {
		return err
	}

	store, err := manifest.Load(inst)
	if err != nil {
		return err
	}

	client, err := newProviderClient(modsource.ProviderModrinth)
	if err != nil {
		return err
	}

	if opts.NoColor {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	model := newTUIModel(
		ctx,
		client,
		download.New(download.WithUserAgent(providerUserAgent())),
		inst,
		store,
	)

	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(a.in),
		tea.WithOutput(a.out),
		tea.WithAltScreen(),
	)

	_, err = program.Run()

	return err
}

func newTUIModel(
	ctx context.Context,
	client modsource.Client,
	downloader *download.Client,
	inst *instance.Instance,
	store *manifest.Store,
) *tuiModel {
	if ctx == nil {
		ctx = context.Background()
	}

	model := &tuiModel{
		ctx:                 ctx,
		client:              client,
		downloader:          downloader,
		inst:                inst,
		store:               store,
		installedList:       newTUIList(),
		searchList:          newTUIList(),
		versionList:         newTUIList(),
		spinner:             spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(tuiSpinnerStyle)),
		installDependencies: true,
	}
	model.query = newTUIInput("sodium, iris, ...", 120)
	model.gameVersion = newTUIInput("1.21.1", 32)
	model.loader = newTUIInput("fabric, forge, ...", 32)
	model.outputView = viewport.New(80, 16)
	model.helpView = viewport.New(80, 12)

	model.resize(tuiDefaultWidth, tuiDefaultHeight)
	model.query.Focus()
	model.syncInputFocus()
	model.status = "Digite um termo e pressione Enter para buscar."

	if err := model.rebuildInstalled(""); err != nil {
		model.setStatus("Falha ao ler o conteúdo instalado: "+err.Error(), true)
	}

	return model
}

func newTUIInput(placeholder string, limit int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = limit
	input.Prompt = ""
	input.Cursor.Style = tuiCursorStyle
	input.PromptStyle = tuiLabelStyle

	return input
}

func newTUIList() list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)
	delegate.SetSpacing(0)
	delegate.Styles = tuiListStyles()

	model := list.New(nil, delegate, 40, 10)
	model.SetShowTitle(false)
	model.SetShowFilter(false)
	model.SetFilteringEnabled(false)
	model.SetShowStatusBar(false)
	model.SetShowPagination(false)
	model.SetShowHelp(false)
	model.DisableQuitKeybindings()

	return model
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tuiSearchMsg:
		return m.handleSearchMsg(msg)
	case tuiProjectMsg:
		return m.handleProjectMsg(msg)
	case tuiInstallMsg:
		return m.handleInstallMsg(msg)
	case tuiUpdateMsg:
		return m.handleUpdateMsg(msg)
	case tuiRemoveMsg:
		return m.handleRemoveMsg(msg)
	default:
		return m, nil
	}
}

func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.overlay != tuiOverlayNone {
		return m.handleOverlayKey(msg)
	}

	if m.busy {
		if key == "esc" {
			m.setStatus("Aguarde a operação atual terminar.", false)
		}
		return m, nil
	}

	switch key {
	case "f1", "alt+1":
		m.switchTab(tuiTabSearch)
		return m, nil
	case "f2", "alt+2":
		m.switchTab(tuiTabInstalled)
		return m, nil
	}

	if !m.typing() {
		switch key {
		case "1":
			m.switchTab(tuiTabSearch)
			return m, nil
		case "2":
			m.switchTab(tuiTabInstalled)
			return m, nil
		case "?":
			m.openHelp()
			return m, nil
		case "q":
			return m, tea.Quit
		case "v":
			if strings.TrimSpace(m.lastOutput) != "" {
				m.openOutput(m.lastOutputTitle, m.lastOutput)
			}
			return m, nil
		}
	}

	if m.tab == tuiTabInstalled {
		return m.handleInstalledKey(msg)
	}

	if m.project != nil {
		return m.handleProjectKey(msg)
	}

	return m.handleSearchKey(msg)
}

func (m tuiModel) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.overlay {
	case tuiOverlayHelp:
		switch key {
		case "esc", "?", "q", "enter":
			m.overlay = tuiOverlayNone
			return m, nil
		}
		var cmd tea.Cmd
		m.helpView, cmd = m.helpView.Update(msg)
		return m, cmd
	case tuiOverlayOutput:
		switch key {
		case "esc", "q", "enter":
			m.overlay = tuiOverlayNone
			return m, nil
		case "o":
			m.overwrite = !m.overwrite
			m.overlay = tuiOverlayNone
			if m.overwrite {
				m.setStatus("Sobrescrita ativada. Repita a operação.", false)
			} else {
				m.setStatus("Sobrescrita desativada.", false)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.outputView, cmd = m.outputView.Update(msg)
		return m, cmd
	case tuiOverlayConfirm:
		switch key {
		case "s", "y", "enter":
			confirm := m.confirm
			m.overlay = tuiOverlayNone
			cmd := m.startRemove(confirm)
			return m, cmd
		case "n", "esc", "q":
			m.overlay = tuiOverlayNone
			m.setStatus("Remoção cancelada.", false)
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m tuiModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "tab" {
		m.cycleFocus(1)
		return m, nil
	}
	if key == "shift+tab" {
		m.cycleFocus(-1)
		return m, nil
	}

	switch m.focus {
	case tuiFocusQuery, tuiFocusGameVersion, tuiFocusLoader:
		switch key {
		case "enter":
			cmd := m.startSearch()
			return m, cmd
		case "esc":
			if len(m.searchProjects) > 0 {
				m.focus = tuiFocusResults
				m.syncInputFocus()
			}
			return m, nil
		}

		var cmd tea.Cmd
		switch m.focus {
		case tuiFocusQuery:
			m.query, cmd = m.query.Update(msg)
		case tuiFocusGameVersion:
			m.gameVersion, cmd = m.gameVersion.Update(msg)
		case tuiFocusLoader:
			m.loader, cmd = m.loader.Update(msg)
		}
		return m, cmd
	case tuiFocusType:
		switch key {
		case "left", "h":
			m.typeIndex = wrapIndex(m.typeIndex-1, len(tuiSearchKindLabels))
			m.searchDone = false
		case "right", "l":
			m.typeIndex = wrapIndex(m.typeIndex+1, len(tuiSearchKindLabels))
			m.searchDone = false
		case "enter":
			cmd := m.startSearch()
			return m, cmd
		case "esc":
			m.focus = tuiFocusResults
			m.syncInputFocus()
		}
		return m, nil
	case tuiFocusChannel:
		switch key {
		case "left", "h":
			m.channelIndex = wrapIndex(m.channelIndex-1, len(tuiChannelLabels))
			m.searchDone = false
		case "right", "l":
			m.channelIndex = wrapIndex(m.channelIndex+1, len(tuiChannelLabels))
			m.searchDone = false
		case "enter":
			cmd := m.startSearch()
			return m, cmd
		case "esc":
			m.focus = tuiFocusResults
			m.syncInputFocus()
		}
		return m, nil
	default:
		return m.handleSearchResultsKey(msg)
	}
}

func (m tuiModel) handleSearchResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.searchList.CursorUp()
	case "down", "j":
		m.searchList.CursorDown()
	case "pgup":
		m.searchList.PrevPage()
	case "pgdown":
		m.searchList.NextPage()
	case "home", "g":
		m.searchList.GoToStart()
	case "end", "G":
		m.searchList.GoToEnd()
	case "/":
		m.focus = tuiFocusQuery
		m.syncInputFocus()
	case "r":
		cmd := m.startSearch()
		return m, cmd
	case "esc":
		m.focus = tuiFocusQuery
		m.syncInputFocus()
	case "enter", "i":
		item, ok := m.searchList.SelectedItem().(tuiProjectItem)
		if !ok {
			return m, nil
		}
		cmd := m.startProject(item.project)
		return m, cmd
	}

	return m, nil
}

func (m tuiModel) handleProjectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearProject()
		m.focus = tuiFocusResults
		m.syncInputFocus()
	case "up", "k":
		m.versionList.CursorUp()
	case "down", "j":
		m.versionList.CursorDown()
	case "pgup":
		m.versionList.PrevPage()
	case "pgdown":
		m.versionList.NextPage()
	case "home", "g":
		m.versionList.GoToStart()
	case "end", "G":
		m.versionList.GoToEnd()
	case "enter", "i":
		cmd := m.startInstall()
		return m, cmd
	case "o":
		m.overwrite = !m.overwrite
		if m.overwrite {
			m.setStatus("Sobrescrita ativada para a próxima instalação.", false)
		} else {
			m.setStatus("Sobrescrita desativada.", false)
		}
	case "d":
		m.installDependencies = !m.installDependencies
		if m.installDependencies {
			m.setStatus("Dependências obrigatórias serão instaladas.", false)
		} else {
			m.setStatus("Dependências obrigatórias não serão instaladas.", false)
		}
	case "r":
		if m.project != nil {
			cmd := m.startProject(*m.project)
			return m, cmd
		}
	}

	return m, nil
}

func (m tuiModel) handleInstalledKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.installedList.CursorUp()
	case "down", "j":
		m.installedList.CursorDown()
	case "pgup":
		m.installedList.PrevPage()
	case "pgdown":
		m.installedList.NextPage()
	case "home", "g":
		m.installedList.GoToStart()
	case "end", "G":
		m.installedList.GoToEnd()
	case "enter":
		item, ok := m.selectedInstalledItem()
		if ok {
			m.openOutput("Detalhes do conteúdo", tuiInstalledDetails(m.inst, item))
		}
	case "u":
		cmd := m.startUpdate(false, false)
		return m, cmd
	case "U", "a":
		cmd := m.startUpdate(false, true)
		return m, cmd
	case "c":
		cmd := m.startUpdate(true, false)
		return m, cmd
	case "C":
		cmd := m.startUpdate(true, true)
		return m, cmd
	case "x", "delete":
		m.openRemoveConfirm()
	case "o":
		m.overwrite = !m.overwrite
		if m.overwrite {
			m.setStatus("Sobrescrita ativada para atualizações.", false)
		} else {
			m.setStatus("Sobrescrita desativada.", false)
		}
	case "d":
		m.installDependencies = !m.installDependencies
		if m.installDependencies {
			m.setStatus("Dependências obrigatórias serão atualizadas automaticamente.", false)
		} else {
			m.setStatus("Dependências obrigatórias não serão atualizadas.", false)
		}
	case "r":
		selection := m.selectedInstalledKey()
		store, err := manifest.Load(m.inst)
		if err != nil {
			m.setStatus("Falha ao recarregar o manifesto: "+err.Error(), true)
			return m, nil
		}
		m.store = store
		if err := m.rebuildInstalled(selection); err != nil {
			m.setStatus("Falha ao recarregar o conteúdo instalado: "+err.Error(), true)
		} else {
			m.setStatus("Manifesto recarregado.", false)
		}
	}

	return m, nil
}

func (m *tuiModel) handleSearchMsg(msg tuiSearchMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.seq {
		return *m, nil
	}
	m.finishBusy()

	m.lastSearchQuery = msg.query
	if msg.err != nil {
		m.setStatus("Falha na busca: "+msg.err.Error(), true)
		m.openOutput("Falha na busca", sanitizeBlock(msg.err.Error()))
		return *m, nil
	}

	m.searchProjects = msg.result.Projects
	m.searchTotal = msg.result.Total
	m.searchDone = true
	items := make([]list.Item, 0, len(msg.result.Projects))
	for _, project := range msg.result.Projects {
		items = append(items, tuiProjectItem{project: project})
	}
	m.searchList.SetItems(items)
	if len(items) > 0 {
		m.searchList.Select(0)
	}
	m.focus = tuiFocusResults
	m.syncInputFocus()

	switch {
	case len(items) == 0:
		m.setStatus("Nenhum projeto encontrado.", false)
	case m.searchTotal > len(items):
		m.setStatus(fmt.Sprintf("%d resultados; exibindo os %d primeiros.", m.searchTotal, len(items)), false)
	default:
		m.setStatus(fmt.Sprintf("%d resultado(s).", len(items)), false)
	}

	return *m, nil
}

func (m *tuiModel) handleProjectMsg(msg tuiProjectMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.seq {
		return *m, nil
	}
	m.finishBusy()

	if msg.err != nil && strings.TrimSpace(msg.project.ID) == "" {
		m.setStatus("Falha ao carregar o projeto: "+msg.err.Error(), true)
		m.openOutput("Falha ao carregar o projeto", sanitizeBlock(msg.err.Error()))
		return *m, nil
	}

	m.setProject(msg.project, msg.versions, msg.filter, msg.channelExplicit)
	if msg.err != nil {
		m.setStatus("Projeto carregado, mas as versões falharam: "+msg.err.Error(), true)
		m.openOutput("Falha ao listar versões", sanitizeBlock(msg.err.Error()))
		return *m, nil
	}
	if len(msg.versions) == 0 {
		m.setStatus("Nenhuma versão compatível foi encontrada.", false)
	} else {
		m.setStatus(fmt.Sprintf("%d versão(ões) compatível(is).", len(msg.versions)), false)
	}

	return *m, nil
}

func (m *tuiModel) handleInstallMsg(msg tuiInstallMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.seq {
		return *m, nil
	}
	m.finishBusy()
	m.lastOutput = msg.output
	m.lastOutputTitle = "Instalação"

	guidance := tuiErrorGuidance(msg.err, m.overwrite)
	if err := m.rebuildInstalled(""); err != nil {
		m.setStatus("Instalação concluída, mas a lista não pôde ser atualizada: "+err.Error(), true)
	} else if msg.err != nil {
		if guidance != "" {
			m.setStatus(guidance, true)
		} else {
			m.setStatus("Falha ao instalar "+msg.projectName+": "+msg.err.Error(), true)
		}
	} else {
		installed := 0
		for _, outcome := range msg.outcomes {
			if !outcome.Skipped {
				installed++
			}
		}
		if installed == 0 {
			m.setStatus("Nada foi instalado; o conteúdo já estava atualizado.", false)
		} else {
			m.setStatus(fmt.Sprintf("Instalação concluída: %d arquivo(s).", installed), false)
		}
	}

	if msg.err != nil {
		content := msg.output
		if strings.TrimSpace(content) == "" {
			content = sanitizeBlock(msg.err.Error())
		} else {
			content += "\n" + sanitizeBlock(msg.err.Error())
		}
		if guidance != "" {
			content += "\n\n" + guidance
		}
		m.openOutput("Falha na instalação", content)
	}

	return *m, nil
}

func (m *tuiModel) handleUpdateMsg(msg tuiUpdateMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.seq {
		return *m, nil
	}
	m.finishBusy()
	m.lastOutput = msg.output
	if msg.check {
		m.lastOutputTitle = "Verificação de atualizações"
	} else {
		m.lastOutputTitle = "Atualização"
	}

	rebuildErr := m.rebuildInstalled("")
	switch {
	case rebuildErr != nil:
		m.setStatus("Operação concluída, mas a lista não pôde ser atualizada: "+rebuildErr.Error(), true)
	case msg.err != nil && msg.check:
		m.setStatus("Verificação concluída com falhas.", true)
	case msg.err != nil:
		m.setStatus("Atualização concluída com falhas.", true)
	case msg.check:
		m.setStatus("Verificação concluída.", false)
	case msg.all:
		m.setStatus("Atualização de todos os itens concluída.", false)
	default:
		m.setStatus("Atualização concluída.", false)
	}

	if msg.check || msg.err != nil {
		content := msg.output
		if msg.err != nil {
			if strings.TrimSpace(content) == "" {
				content = sanitizeBlock(msg.err.Error())
			} else {
				content += "\n" + sanitizeBlock(msg.err.Error())
			}
			if guidance := tuiErrorGuidance(msg.err, m.overwrite); guidance != "" {
				content += "\n\n" + guidance
			}
		}
		if strings.TrimSpace(content) == "" {
			content = "Nenhuma saída."
		}
		m.openOutput(m.lastOutputTitle, content)
	}

	return *m, nil
}

func (m *tuiModel) handleRemoveMsg(msg tuiRemoveMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.seq {
		return *m, nil
	}
	m.finishBusy()
	m.lastOutput = msg.output
	m.lastOutputTitle = "Remoção"

	if err := m.rebuildInstalled(""); err != nil {
		m.setStatus("Remoção concluída, mas a lista não pôde ser atualizada: "+err.Error(), true)
	} else if msg.err != nil {
		m.setStatus("Falha ao remover: "+msg.err.Error(), true)
	} else {
		m.setStatus("Conteúdo removido.", false)
	}

	if msg.err != nil {
		content := msg.output
		if strings.TrimSpace(content) == "" {
			content = sanitizeBlock(msg.err.Error())
		} else {
			content += "\n" + sanitizeBlock(msg.err.Error())
		}
		m.openOutput("Falha na remoção", content)
	}

	return *m, nil
}

func (m *tuiModel) startSearch() tea.Cmd {
	if m.client == nil {
		m.setStatus("Cliente de conteúdo não inicializado.", true)
		return nil
	}

	query := m.searchQuery()
	m.searchDone = false
	seq := m.beginBusy("Buscando projetos...")
	client := m.client
	ctx := m.ctx

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		result, err := client.Search(ctx, query)
		return tuiSearchMsg{seq: seq, query: query, result: result, err: err}
	})
}

func (m *tuiModel) startProject(project modsource.Project) tea.Cmd {
	if m.client == nil {
		m.setStatus("Cliente de conteúdo não inicializado.", true)
		return nil
	}

	reference := projectIdentifier(project)
	if reference == "" {
		reference = strings.TrimSpace(project.ID)
	}
	if reference == "" {
		m.setStatus("O projeto não possui slug ou ID.", true)
		return nil
	}

	filter := m.versionFilter()
	channelExplicit := m.channelExplicit()
	seq := m.beginBusy("Carregando projeto...")
	client := m.client
	ctx := m.ctx

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		details, err := client.GetProject(ctx, reference)
		if err != nil {
			return tuiProjectMsg{seq: seq, err: err}
		}

		projectID := strings.TrimSpace(details.ID)
		if projectID == "" {
			projectID = reference
			details.ID = reference
		}
		versions, versionErr := client.ListVersions(ctx, projectID, filter)
		if versionErr == nil {
			sort.SliceStable(versions, func(i, j int) bool {
				return versions[i].PublishedAt.After(versions[j].PublishedAt)
			})
		}

		return tuiProjectMsg{
			seq:             seq,
			project:         details,
			versions:        versions,
			filter:          filter,
			channelExplicit: channelExplicit,
			err:             versionErr,
		}
	})
}

func (m *tuiModel) startInstall() tea.Cmd {
	if m.project == nil {
		m.setStatus("Nenhum projeto selecionado.", true)
		return nil
	}
	if _, err := installDirectoryName(m.project.Type); err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}

	item, ok := m.versionList.SelectedItem().(tuiVersionItem)
	if !ok {
		m.setStatus("Nenhuma versão selecionada.", true)
		return nil
	}
	if _, err := item.version.PrimaryFile(); err != nil {
		m.setStatus("A versão selecionada não possui arquivo instalável: "+err.Error(), true)
		return nil
	}

	seq := m.beginBusy("Instalando...")
	client := m.client
	downloader := m.downloader
	store := m.store
	inst := m.inst
	ctx := m.ctx
	projectID := m.project.ID
	projectName := projectDisplayName(*m.project)
	versionID := item.version.ID
	filter := m.projectFilter
	channelExplicit := m.projectChannelExplicit
	overwrite := m.overwrite
	resolveDependencies := m.installDependencies

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		var output bytes.Buffer
		installer := newContentInstaller(
			client,
			downloader,
			store,
			inst,
			filter,
			channelExplicit,
			overwrite,
			false,
			resolveDependencies,
			io.Discard,
		)
		outcomes, err := installer.InstallProject(ctx, projectID, versionID)
		writeErr := writeInstallOutcomes(&output, outcomes, false)
		if writeErr != nil {
			err = errors.Join(err, writeErr)
		}

		return tuiInstallMsg{
			seq:         seq,
			projectName: projectName,
			outcomes:    outcomes,
			output:      output.String(),
			err:         err,
		}
	})
}

func (m *tuiModel) startUpdate(check, all bool) tea.Cmd {
	if m.client == nil {
		m.setStatus("Cliente de conteúdo não inicializado.", true)
		return nil
	}
	if m.store == nil {
		m.setStatus("Manifesto não inicializado.", true)
		return nil
	}

	var entries []manifest.Entry
	if all {
		entries = m.store.Entries()
	} else {
		item, ok := m.selectedInstalledItem()
		if !ok || !item.tracked {
			m.setStatus("Selecione um item rastreado para atualizar ou verificar.", true)
			return nil
		}
		entries = []manifest.Entry{item.entry}
	}
	if len(entries) == 0 {
		m.setStatus("Nenhum conteúdo rastreado nesta instância.", false)
		return nil
	}

	label := "Atualizando..."
	if check {
		label = "Verificando atualizações..."
	}
	seq := m.beginBusy(label)
	client := m.client
	downloader := m.downloader
	store := m.store
	inst := m.inst
	ctx := m.ctx
	overwrite := m.overwrite
	resolveDependencies := m.installDependencies

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		var output strings.Builder
		var updateErrs []error
		opts := &updateOptions{
			check:          check,
			overwrite:      overwrite,
			noDependencies: !resolveDependencies,
		}

		for index, entry := range entries {
			if all {
				if index > 0 {
					output.WriteString("\n")
				}
				fmt.Fprintf(&output, "== %s ==\n", sanitizeText(entry.ProjectLabel()))
			}
			if err := updateEntry(
				ctx,
				client,
				downloader,
				store,
				inst,
				entry,
				opts,
				nil,
				false,
				&output,
				io.Discard,
			); err != nil {
				updateErrs = append(updateErrs, fmt.Errorf(
					"falha ao atualizar %q: %w",
					sanitizeText(entry.ProjectLabel()),
					err,
				))
			}
		}

		return tuiUpdateMsg{
			seq:    seq,
			check:  check,
			all:    all,
			output: output.String(),
			err:    errors.Join(updateErrs...),
		}
	})
}

func (m *tuiModel) startRemove(confirm tuiConfirm) tea.Cmd {
	if m.store == nil || m.inst == nil {
		m.setStatus("Instância ou manifesto não inicializado.", true)
		return nil
	}

	seq := m.beginBusy("Removendo...")
	store := m.store
	inst := m.inst
	query := confirm.query
	kind := confirm.entry

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		var output bytes.Buffer
		target, err := resolveRemovalTarget(inst, store, query, kind)
		if err != nil {
			return tuiRemoveMsg{seq: seq, err: err}
		}

		result, removeErr := removeTarget(inst, store, target)
		var writeErr error
		if removeErr == nil || result.existed {
			writeErr = writeRemovalResult(&output, result)
		}

		return tuiRemoveMsg{
			seq:    seq,
			output: output.String(),
			err:    errors.Join(removeErr, writeErr),
		}
	})
}

func (m *tuiModel) openRemoveConfirm() {
	item, ok := m.selectedInstalledItem()
	if !ok {
		m.setStatus("Nenhum item selecionado.", true)
		return
	}

	label := item.Title()
	if item.tracked {
		label = fmt.Sprintf("%s %s (%s)", displayProjectType(item.entry.Type), item.entry.ProjectLabel(), item.entry.VersionLabel())
	}

	m.confirm = tuiConfirm{
		kind:  tuiConfirmRemove,
		title: "Confirmar remoção",
		body:  "Remover " + label + "?\n\nArquivo: " + item.Key(),
		query: item.Key(),
		entry: item.kind(),
	}
	m.overlay = tuiOverlayConfirm
}

func (m *tuiModel) setProject(
	project modsource.Project,
	versions []modsource.Version,
	filter modsource.VersionFilter,
	channelExplicit bool,
) {
	copy := project
	m.project = &copy
	m.versions = append([]modsource.Version(nil), versions...)
	m.projectFilter = filter
	m.projectChannelExplicit = channelExplicit

	items := make([]list.Item, 0, len(versions))
	for _, version := range versions {
		items = append(items, tuiVersionItem{version: version})
	}
	m.versionList.SetItems(items)
	if len(items) > 0 {
		m.versionList.Select(0)
	}
	m.syncInputFocus()
}

func (m *tuiModel) clearProject() {
	m.project = nil
	m.versions = nil
	m.projectFilter = modsource.VersionFilter{}
	m.projectChannelExplicit = false
	m.versionList.SetItems(nil)
}

func (m *tuiModel) rebuildInstalled(selection string) error {
	oldIndex := m.installedList.Index()
	items, scanErr := buildTUIInstalledItems(m.inst, m.store)
	m.installed = items

	listItems := make([]list.Item, 0, len(items))
	selectedIndex := -1
	for index, item := range items {
		listItems = append(listItems, item)
		if selection != "" && item.Key() == selection {
			selectedIndex = index
		}
	}
	m.installedList.SetItems(listItems)

	switch {
	case selectedIndex >= 0:
		m.installedList.Select(selectedIndex)
	case len(items) > 0:
		if oldIndex < 0 {
			oldIndex = 0
		}
		if oldIndex >= len(items) {
			oldIndex = len(items) - 1
		}
		m.installedList.Select(oldIndex)
	}

	return scanErr
}

func (m *tuiModel) selectedInstalledItem() (tuiInstalledItem, bool) {
	item, ok := m.installedList.SelectedItem().(tuiInstalledItem)
	return item, ok
}

func (m *tuiModel) selectedInstalledKey() string {
	item, ok := m.selectedInstalledItem()
	if !ok {
		return ""
	}

	return item.Key()
}

func (m *tuiModel) beginBusy(label string) int {
	m.busy = true
	m.busyLabel = sanitizeText(label)
	m.seq++
	m.overlay = tuiOverlayNone
	m.statusError = false
	m.syncInputFocus()

	return m.seq
}

func (m *tuiModel) finishBusy() {
	m.busy = false
	m.busyLabel = ""
	m.syncInputFocus()
}

func (m *tuiModel) setStatus(text string, isError bool) {
	m.status = sanitizeText(text)
	m.statusError = isError
}

func tuiErrorGuidance(err error, overwrite bool) string {
	if err == nil || overwrite {
		return ""
	}
	if errors.Is(err, download.ErrTargetExists) || strings.Contains(strings.ToLower(err.Error()), "overwrite") {
		return "Pressione o para ativar a sobrescrita e tente novamente."
	}

	return ""
}

func (m *tuiModel) openHelp() {
	m.helpView.SetContent(tuiHelpContent(maxInt(10, m.helpView.Width)))
	m.helpView.GotoTop()
	m.overlay = tuiOverlayHelp
}

func (m *tuiModel) openOutput(title, content string) {
	title = sanitizeText(title)
	if title == "" {
		title = "Saída"
	}
	content = strings.TrimRight(content, "\n")
	if strings.TrimSpace(content) == "" {
		content = "Nenhuma saída."
	}

	m.lastOutputTitle = title
	m.lastOutput = content
	m.outputView.SetContent(content)
	m.outputView.GotoTop()
	m.overlay = tuiOverlayOutput
}

func (m *tuiModel) switchTab(tab tuiTab) {
	m.tab = tab
	m.overlay = tuiOverlayNone
	m.statusError = false

	switch tab {
	case tuiTabSearch:
		switch {
		case m.project != nil:
			m.focus = tuiFocusResults
			m.status = ""
		case len(m.searchProjects) > 0:
			m.focus = tuiFocusResults
			m.status = fmt.Sprintf("%d resultado(s).", len(m.searchProjects))
		default:
			m.focus = tuiFocusQuery
			m.status = "Digite um termo e pressione Enter para buscar."
		}
	case tuiTabInstalled:
		m.status = fmt.Sprintf("%d item(ns) na instalação.", len(m.installed))
	}

	m.syncInputFocus()
}

func (m *tuiModel) cycleFocus(delta int) {
	order := []tuiFocus{
		tuiFocusQuery,
		tuiFocusGameVersion,
		tuiFocusLoader,
		tuiFocusType,
		tuiFocusChannel,
		tuiFocusResults,
	}

	index := 0
	for i, focus := range order {
		if focus == m.focus {
			index = i
			break
		}
	}
	index = wrapIndex(index+delta, len(order))
	m.focus = order[index]
	m.syncInputFocus()
}

func (m *tuiModel) syncInputFocus() {
	searchActive := m.tab == tuiTabSearch && m.project == nil && !m.busy
	if searchActive && m.focus == tuiFocusQuery {
		m.query.Focus()
	} else {
		m.query.Blur()
	}
	if searchActive && m.focus == tuiFocusGameVersion {
		m.gameVersion.Focus()
	} else {
		m.gameVersion.Blur()
	}
	if searchActive && m.focus == tuiFocusLoader {
		m.loader.Focus()
	} else {
		m.loader.Blur()
	}
}

func (m *tuiModel) typing() bool {
	if m.tab != tuiTabSearch || m.project != nil {
		return false
	}

	switch m.focus {
	case tuiFocusQuery, tuiFocusGameVersion, tuiFocusLoader:
		return true
	default:
		return false
	}
}

func (m tuiModel) searchFormHasInput() bool {
	return strings.TrimSpace(m.query.Value()) != "" ||
		strings.TrimSpace(m.gameVersion.Value()) != "" ||
		strings.TrimSpace(m.loader.Value()) != "" ||
		m.typeIndex != 0
}

func (m tuiModel) searchQuery() modsource.SearchQuery {
	return modsource.SearchQuery{
		Text:        strings.TrimSpace(m.query.Value()),
		Kind:        tuiSearchKinds[m.typeIndex],
		GameVersion: strings.TrimSpace(m.gameVersion.Value()),
		Loader:      strings.TrimSpace(m.loader.Value()),
		Sort:        modsource.SortRelevance,
		Limit:       tuiSearchLimit,
	}
}

func (m tuiModel) versionFilter() modsource.VersionFilter {
	return modsource.VersionFilter{
		GameVersion: strings.TrimSpace(m.gameVersion.Value()),
		Loader:      strings.TrimSpace(m.loader.Value()),
		Channels:    m.selectedChannels(),
	}
}

func (m tuiModel) selectedChannels() []modsource.ReleaseChannel {
	switch m.channelIndex {
	case 0:
		return []modsource.ReleaseChannel{modsource.ChannelRelease}
	case 1:
		return []modsource.ReleaseChannel{modsource.ChannelBeta}
	case 2:
		return []modsource.ReleaseChannel{modsource.ChannelAlpha}
	default:
		return nil
	}
}

func (m tuiModel) channelExplicit() bool {
	return m.channelIndex >= 0 && m.channelIndex < 3
}

func (m *tuiModel) resize(width, height int) {
	if width < 20 {
		width = 20
	}
	if height < 10 {
		height = 10
	}

	m.width = width
	m.height = height
	contentWidth := maxInt(10, width-4)

	m.query.Width = maxInt(12, contentWidth-14)
	m.gameVersion.Width = 14
	m.loader.Width = 16

	m.searchList.SetSize(contentWidth, maxInt(3, height-11))
	m.installedList.SetSize(contentWidth, maxInt(3, height-6))
	m.versionList.SetSize(contentWidth, maxInt(3, height-12))
	m.outputView.Width = maxInt(20, width-8)
	m.outputView.Height = maxInt(4, height-12)
	m.helpView.Width = maxInt(10, width-12)
	m.helpView.Height = maxInt(3, height-16)

	if m.overlay == tuiOverlayHelp {
		m.helpView.SetContent(tuiHelpContent(m.helpView.Width))
	}
}

func wrapIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	index %= length
	if index < 0 {
		index += length
	}

	return index
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
