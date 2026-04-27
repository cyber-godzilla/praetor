package ui

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/textutil"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Tab constants for special tab kinds (looked up by kind, not index).
// Custom tabs are accessed by dynamic index.

// EventMsg wraps a batch of game events for delivery to the TUI.
// The event bridge in main.go drains all available events from the client
// channel before sending a single EventMsg, so that Bubbletea renders once
// per batch instead of once per line.
type EventMsg struct {
	Events []types.Event
}

// AuthResultMsg is sent after the HTTP login attempt completes.
type AuthResultMsg struct {
	Success bool
	Error   string
}

// CredentialPromptMsg triggers the credential storage prompt after login.
type CredentialPromptMsg struct {
	Username      string
	Password      string
	AlreadyStored bool
}

// SidebarToggleMsg is sent when the sidebar is toggled so config can be saved.
type SidebarToggleMsg struct {
	Open bool
}

// OpenModePickerMsg provides the list of all available modes to the mode picker.
type OpenModePickerMsg struct {
	AllModes []string
}

// ModesAvailableMsg signals whether the engine has any modes loaded. Used
// to hide menu items that depend on modes existing.
type ModesAvailableMsg struct {
	Available bool
}

// kittyDeleteAll clears all Kitty graphics images from the terminal.
const kittyDeleteAll = "\033_Ga=d,d=A,q=2;\033\\"

// graphicsClear returns the Kitty delete-all escape when running in Kitty
// graphics mode, and an empty string otherwise. This prevents leaking
// Kitty-specific APC sequences to terminals that don't support them.
func (a App) graphicsClear() string {
	if a.graphicsMode == graphics.ModeKitty {
		return kittyDeleteAll
	}
	return ""
}

// appState tracks the current screen.
type appState int

const (
	stateSplash               appState = iota // splash screen on startup
	stateAccountSelect                        // picking from stored accounts
	stateLogin                                // entering new credentials
	stateAuthenticating                       // login submitted, waiting for result
	stateCredentialPrompt                     // asking whether to save credentials
	stateGame                                 // connected, showing game UI
	stateMenu                                 // overlay menu
	stateModePicker                           // editing quick-cycle mode list
	stateHighlights                           // editing highlight patterns
	stateHelp                                 // help screen
	stateTabEditor                            // custom tab editor
	statePersistentData                       // persistent data viewer
	stateScriptDirs                           // script directory management
	statePriorityCmds                         // priority command management
	stateNotificationSettings                 // notification settings editor
	stateWikiMenu                             // browsing wiki bookmarks
)

// App is the root Bubbletea model composing all TUI components.
type App struct {
	width  int
	height int

	activeTab      int
	sidebarOpen    bool
	sidebarWidth   int
	sidebarVisible bool // computed: sidebarOpen AND terminal large enough
	sidebarCompact bool // computed: only show minimap+compass (no bars/lighting)
	state          appState
	authError      string // error message from failed auth
	pendingQuit    bool   // true after first Ctrl+C, quit on second
	debugMode      bool   // show Debug tab (--debug flag)
	scrollback     int

	tabs          []TabDef
	metrics       MetricsPane
	debug         DebugPane
	tabEditor     TabEditor
	sidebar       *Sidebar
	status        StatusBar
	input         Input
	login         LoginScreen
	accountSelect AccountSelect
	menu          Menu
	quickCycle    QuickCycle
	modePicker    ModePicker
	highlightsMgr HighlightsManager
	highlights    []config.HighlightConfig
	help          HelpScreen
	colorWords    bool
	echoTyped     bool
	echoScript    bool
	autoReconnect bool
	hideIPs       bool
	gameLogs      bool
	logPath       string
	unread        []bool

	splash                  Splash
	credentialPrompt        CredentialPrompt
	persistentData          PersistentDataScreen
	scriptDirsScreen        ScriptDirsScreen
	scriptDirsList          []string
	priorityCmdsScreen      PriorityCmdsScreen
	priorityCmdsList        []string
	notificationSettings    NotificationSettingsScreen
	notificationSettingsCfg config.DesktopNotificationsConfig
	wikiMenu                WikiMenu
	modesAvailable          bool
	version                 string
	graphicsMode            graphics.Mode
}

// NewApp creates a new App with the specified initial configuration.
// defaultTab should be one of "all", "combat", "social", "metrics".
// accounts is the list of stored usernames; if non-empty, the app starts
// on the account selection screen; otherwise it starts on the login screen.
func NewApp(sidebarOpen bool, defaultTab string, scrollback int, accounts []string, sidebarWidth int, minimapScale float64, minimapHeight int, quickCycleModes []string, highlights []config.HighlightConfig, debugMode bool, colorWords bool, customTabs []config.CustomTabConfig, version string, autoReconnect bool, hideIPs bool, echoTyped bool, echoScript bool, gameLogs bool, logPath string, scriptDirs []string, priorityCmds []string, notifyCfg config.DesktopNotificationsConfig, graphicsMode graphics.Mode) App {
	tabs := BuildTabs(scrollback, debugMode, customTabs)
	tab := 0 // default to All

	initialState := stateSplash

	if sidebarWidth <= 0 {
		sidebarWidth = 40
	}

	a := App{
		activeTab:    tab,
		sidebarOpen:  sidebarOpen,
		sidebarWidth: sidebarWidth,
		debugMode:    debugMode,
		scrollback:   scrollback,
		state:        initialState,

		tabs:          tabs,
		metrics:       NewMetricsPane(),
		debug:         NewDebugPane(),
		sidebar:       newSidebarPtr(minimapScale, minimapHeight, graphicsMode),
		status:        NewStatusBar(),
		input:         NewInput(),
		login:         NewLoginScreen(),
		accountSelect: NewAccountSelect(accounts),
		menu:          NewMenu(colorWords, echoTyped, echoScript, autoReconnect, hideIPs, gameLogs, logPath, false),
		quickCycle:    NewQuickCycle(quickCycleModes),
		highlights:    highlights,
		colorWords:    colorWords,
		echoTyped:     echoTyped,
		echoScript:    echoScript,
		autoReconnect: autoReconnect,
		hideIPs:       hideIPs,
		gameLogs:      gameLogs,
		logPath:       logPath,
		unread:        make([]bool, len(tabs)),

		splash:                  NewSplash(version),
		wikiMenu:                NewWikiMenu(),
		scriptDirsList:          scriptDirs,
		priorityCmdsList:        priorityCmds,
		notificationSettingsCfg: notifyCfg,
		version:                 version,
		graphicsMode:            graphicsMode,
	}
	a.login.hasAccounts = len(accounts) > 0
	return a
}

// SetLoggedIn transitions to the main game view.
func (a *App) SetLoggedIn(loggedIn bool) {
	if loggedIn {
		a.state = stateGame
	} else {
		a.state = stateLogin
	}
}

// SetPersistentDataMessage sets a status message on the persistent data screen.
func (a *App) SetPersistentDataMessage(msg string) {
	a.persistentData.SetMessage(msg)
}

// Init returns the initial command (cursor blink for the input/login).
func (a App) Init() tea.Cmd {
	switch a.state {
	case stateSplash:
		return a.splash.Init()
	case stateGame:
		return a.input.Init()
	case stateLogin:
		return a.login.Init()
	default:
		return nil
	}
}

// Update handles all messages for the App.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.recalcLayout()
		return a, nil

	case tea.MouseMsg:
		if a.state == stateGame {
			switch msg.Type {
			case tea.MouseWheelUp:
				if a.tabs[a.activeTab].Kind == TabKindDebug {
					a.debug.ScrollUp(3)
				} else {
					a.tabs[a.activeTab].Pane.ScrollUp(3)
				}
			case tea.MouseWheelDown:
				if a.tabs[a.activeTab].Kind == TabKindDebug {
					a.debug.ScrollDown(3)
				} else {
					a.tabs[a.activeTab].Pane.ScrollDown(3)
				}
			}
		}
		return a, nil

	case tea.KeyMsg:
		switch a.state {
		case stateSplash:
			if !a.splash.showHint {
				return a, nil
			}
			if len(a.accountSelect.accounts) > 0 {
				a.state = stateAccountSelect
			} else {
				a.state = stateLogin
				return a, a.login.Init()
			}
			return a, nil
		case stateAccountSelect:
			return a.updateAccountSelect(msg)
		case stateLogin:
			return a.updateLogin(msg)
		case stateAuthenticating:
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a, nil
		case stateCredentialPrompt:
			var cmd tea.Cmd
			a.credentialPrompt, cmd = a.credentialPrompt.Update(msg)
			return a, cmd
		case stateMenu:
			return a.updateMenu(msg)
		case stateModePicker:
			return a.updateModePicker(msg)
		case stateHighlights:
			return a.updateHighlights(msg)
		case stateHelp:
			return a.updateHelp(msg)
		case stateTabEditor:
			return a.updateTabEditor(msg)
		case statePersistentData:
			var cmd tea.Cmd
			a.persistentData, cmd = a.persistentData.Update(msg)
			return a, cmd
		case stateScriptDirs:
			var cmd tea.Cmd
			a.scriptDirsScreen, cmd = a.scriptDirsScreen.Update(msg)
			return a, cmd
		case statePriorityCmds:
			var cmd tea.Cmd
			a.priorityCmdsScreen, cmd = a.priorityCmdsScreen.Update(msg)
			return a, cmd
		case stateNotificationSettings:
			var cmd tea.Cmd
			a.notificationSettings, cmd = a.notificationSettings.Update(msg)
			return a, cmd
		case stateWikiMenu:
			return a.updateWikiMenu(msg)
		case stateGame:
			return a.updateMain(msg)
		}

	case splashTickMsg:
		a.splash, _ = a.splash.Update(msg)
		return a, nil

	case SetModeMsg:
		// Handled by main.go wrapper — it calls client.Engine.SetMode.
		// The ModeChangeEvent from the engine will update sidebar/status.
		return a, nil

	case MenuCloseMsg:
		a.state = stateGame
		return a, a.input.Focus()

	case MenuQuickCycleMsg:
		// Handled by main.go — it fetches mode names and sends OpenModePickerMsg.
		return a, nil

	case MenuColorWordsMsg:
		a.colorWords = !a.colorWords
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuEchoTypedMsg:
		a.echoTyped = !a.echoTyped
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuEchoScriptMsg:
		a.echoScript = !a.echoScript
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuAutoReconnectMsg:
		a.autoReconnect = !a.autoReconnect
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuHideIPsMsg:
		a.hideIPs = !a.hideIPs
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuGameLogsMsg:
		a.gameLogs = !a.gameLogs
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuLogPathMsg:
		a.logPath = msg.Path
		cursor := a.menu.cursor
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.cursor = cursor
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuHighlightsMsg:
		a.highlightsMgr = NewHighlightsManager(a.highlights)
		a.highlightsMgr.SetSize(a.width, a.height)
		a.state = stateHighlights
		return a, nil

	case HelpCloseMsg:
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case HelpSearchMsg:
		// Handled by wrapper — sends ?query to server or opens wiki.
		return a, nil

	case HighlightsCloseMsg:
		a.highlights = msg.Highlights
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case OpenModePickerMsg:
		a.modePicker = NewModePicker(msg.AllModes, a.quickCycle.Modes())
		a.modePicker.SetSize(a.width, a.height)
		a.state = stateModePicker
		return a, nil

	case ModePickerCloseMsg:
		a.quickCycle.SetModes(msg.Modes)
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuReloadScriptsMsg:
		// Stay on menu — main.go wrapper handles the actual reload.
		return a, nil

	case ScriptsReloadedMsg:
		if msg.Error != nil {
			a.menu.SetMessage("Reload failed: " + msg.Error.Error())
		} else {
			a.menu.SetMessage("Scripts reloaded successfully")
		}
		return a, nil

	case ModesAvailableMsg:
		a.modesAvailable = msg.Available
		// Rebuild menu so the Quick-Cycle Modes item appears/disappears.
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		return a, nil

	case MenuScriptDirsMsg:
		a.scriptDirsScreen = NewScriptDirsScreen(a.scriptDirsList)
		a.scriptDirsScreen.SetSize(a.width, a.height)
		a.state = stateScriptDirs
		return a, nil

	case ScriptDirsCloseMsg:
		if msg.Changed {
			a.scriptDirsList = msg.Dirs
		}
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuPriorityCmdsMsg:
		a.priorityCmdsScreen = NewPriorityCmdsScreen(a.priorityCmdsList)
		a.priorityCmdsScreen.SetSize(a.width, a.height)
		a.state = statePriorityCmds
		return a, nil

	case PriorityCmdsCloseMsg:
		if msg.Changed {
			a.priorityCmdsList = msg.Cmds
		}
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuNotificationSettingsMsg:
		a.notificationSettings = NewNotificationSettingsScreen(a.notificationSettingsCfg)
		a.notificationSettings.SetSize(a.width, a.height)
		a.state = stateNotificationSettings
		return a, nil

	case NotificationSettingsCloseMsg:
		a.notificationSettingsCfg = msg.Config
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuWikiMsg:
		a.wikiMenu = NewWikiMenu()
		a.wikiMenu.SetSize(a.width, a.height)
		a.state = stateWikiMenu
		return a, nil

	case WikiMenuCloseMsg:
		a.state = stateGame
		return a, a.input.Focus()

	case WikiOpenMsg:
		// Transition back to game; re-emit so the wrapper can open the browser.
		a.state = stateGame
		return a, func() tea.Msg { return msg }

	case MenuTabsMsg:
		a.tabEditor = NewTabEditor(TabsToConfig(a.tabs))
		a.tabEditor.SetSize(a.width, a.height)
		a.state = stateTabEditor
		return a, nil

	case TabEditorCloseMsg:
		// Rebuild tabs, preserving scrollback for surviving tabs.
		oldTabs := a.tabs
		a.tabs = BuildTabs(a.scrollback, a.debugMode, msg.Tabs)
		for i := range a.tabs {
			for _, old := range oldTabs {
				if a.tabs[i].Kind == old.Kind && a.tabs[i].Name == old.Name {
					a.tabs[i].Pane = old.Pane
					break
				}
			}
		}
		a.unread = make([]bool, len(a.tabs))
		if a.activeTab >= len(a.tabs) {
			a.activeTab = 0
		}
		a.recalcLayout()
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuPersistentDataMsg:
		// Handled by main.go — it snapshots data and sends PersistentDataSnapshotMsg.
		return a, nil

	case PersistentDataSnapshotMsg:
		a.persistentData = NewPersistentDataScreen(msg.Username, msg.Keys)
		a.persistentData.SetSize(a.width, a.height)
		a.state = statePersistentData
		return a, nil

	case PersistentDataCloseMsg:
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case MenuQuitMsg:
		return a, tea.Quit

	case CredentialPromptMsg:
		a.credentialPrompt = NewCredentialPrompt(msg.Username, msg.Password, msg.AlreadyStored)
		a.credentialPrompt.SetSize(a.width, a.height)
		a.state = stateCredentialPrompt
		return a, nil

	case AuthResultMsg:
		if msg.Success {
			// Auth succeeded — main.go will connect WebSocket and send
			// a ConnectedEvent, which transitions us to stateGame.
			a.state = stateGame
			a.authError = ""
			return a, a.input.Focus()
		}
		// Auth failed — return to previous screen with error.
		a.authError = msg.Error
		a.login.SetError(msg.Error)
		a.state = stateLogin
		return a, nil

	case AccountSelectMsg:
		// Handled by main.go — transition to authenticating state
		a.state = stateAuthenticating
		return a, nil

	case AddAccountMsg:
		// Switch to login form
		a.state = stateLogin
		return a, a.login.Init()

	case DeleteAccountMsg:
		// Handled by main.go — it will send an updated account list
		return a, nil

	case AccountListUpdatedMsg:
		// Refresh the account list after a deletion.
		a.login.hasAccounts = len(msg.Accounts) > 0
		if len(msg.Accounts) == 0 {
			a.state = stateLogin
			return a, a.login.Init()
		}
		a.accountSelect.SetAccounts(msg.Accounts)
		a.state = stateAccountSelect
		return a, nil

	case EventMsg:
		return a.handleEvent(msg)

	case InputSubmitMsg:
		// Forward to caller (main.go handles this)
		return a, nil

	case LoginSubmitMsg:
		// Transition to authenticating state; main.go handles the actual auth
		a.state = stateAuthenticating
		return a, nil
	}

	// Forward unhandled messages to focused component
	switch a.state {
	case stateLogin:
		var cmd tea.Cmd
		a.login, cmd = a.login.Update(msg)
		return a, cmd
	case stateGame:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		return a, cmd
	}

	return a, nil
}

// AccountListUpdatedMsg is sent after an account is deleted to refresh the list.
type AccountListUpdatedMsg struct {
	Accounts []string
}

// updateAccountSelect handles key messages on the account selection screen.
func (a App) updateAccountSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return a, tea.Quit
	}

	var cmd tea.Cmd
	a.accountSelect, cmd = a.accountSelect.Update(msg)
	return a, cmd
}

// updateLogin handles key messages on the login screen.
func (a App) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return a, tea.Quit
	case tea.KeyEscape:
		// Go back to account select if there are stored accounts.
		if len(a.accountSelect.accounts) > 0 {
			a.state = stateAccountSelect
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.login, cmd = a.login.Update(msg)
	return a, cmd
}

// Reserved Alt keys that conflict with terminal emulators or readline.
// These must NOT be bound to application actions.
//
//	Alt+A,B,C,D — VT100 cursor control + readline
//	Alt+F       — readline forward word + Ghostty
//	Alt+H,J,K   — VT100 cursor control
//	Alt+L,U,R,T,Y — readline word manipulation
//	Alt+Z       — terminal identify
//	Alt+[       — CSI escape sequence
//
// Safe Alt keys for application use:
//
//	Alt+E, G, I, M, N, O, P, Q, S, V, W, X
var reservedAltKeys = map[rune]bool{
	'a': true, 'b': true, 'c': true, 'd': true,
	'f': true, 'h': true, 'j': true, 'k': true,
	'l': true, 'r': true, 't': true, 'u': true,
	'y': true, 'z': true, '[': true,
}

// updateMain handles key messages in the main game view.
//
// Key bindings:
//
//	Tab         → next tab
//	Shift+Tab   → previous tab
//	Alt+1..5    → switch to tab N
//	Alt+S       → toggle sidebar
//	Esc         → open menu
//	Ctrl+C      → clear input, or quit confirmation
//	PgUp/PgDn   → scroll output
//	All other keys → forwarded to input field
func (a App) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key other than Ctrl+C cancels the pending quit.
	if msg.Type != tea.KeyCtrlC && a.pendingQuit {
		a.pendingQuit = false
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		if a.input.textinput.Value() != "" {
			a.input.textinput.SetValue("")
			a.pendingQuit = false
			return a, nil
		}
		if a.pendingQuit {
			return a, tea.Quit
		}
		a.pendingQuit = true
		return a, nil

	case tea.KeyEscape:
		a.state = stateMenu
		a.menu = NewMenu(a.colorWords, a.echoTyped, a.echoScript, a.autoReconnect, a.hideIPs, a.gameLogs, a.logPath, a.modesAvailable)
		a.menu.SetSize(a.width, a.height)
		return a, nil

	case tea.KeyTab:
		a.nextVisibleTab(1)
		return a, nil

	case tea.KeyShiftTab:
		a.nextVisibleTab(-1)
		return a, nil

	case tea.KeyPgUp:
		if a.tabs[a.activeTab].Kind == TabKindDebug {
			a.debug.ScrollUp(a.contentHeight() / 2)
		} else {
			a.tabs[a.activeTab].Pane.ScrollUp(a.contentHeight() / 2)
		}
		return a, nil

	case tea.KeyPgDown:
		if a.tabs[a.activeTab].Kind == TabKindDebug {
			a.debug.ScrollDown(a.contentHeight() / 2)
		} else {
			a.tabs[a.activeTab].Pane.ScrollDown(a.contentHeight() / 2)
		}
		return a, nil
	}

	// Handle Alt+key combinations.
	if msg.Alt && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]

		// Alt+number: switch to Nth visible tab (1-9, 0=10th).
		if r >= '1' && r <= '9' {
			a.switchToVisibleTab(int(r - '1'))
			return a, nil
		}
		if r == '0' {
			a.switchToVisibleTab(9)
			return a, nil
		}

		// Alt+letter: application shortcuts.
		// Reserved keys are passed through to the terminal/input.
		lower := r
		if lower >= 'A' && lower <= 'Z' {
			lower = lower + 32 // to lowercase
		}

		if !reservedAltKeys[lower] {
			switch lower {
			case 's':
				// Alt+S: toggle sidebar
				a.sidebarOpen = !a.sidebarOpen
				a.recalcLayout()
				open := a.sidebarOpen
				return a, func() tea.Msg { return SidebarToggleMsg{Open: open} }

			case 'm':
				// Alt+M: quick-cycle to next mode
				mode := a.quickCycle.Next()
				return a, func() tea.Msg { return SetModeMsg{Mode: mode} }

			case 'x':
				// Alt+X: disable all automation
				return a, func() tea.Msg { return SetModeMsg{Mode: "disable"} }
			}
		}

		// Reserved Alt keys and unbound Alt keys: don't forward to input.
		// They'd just insert garbage characters.
		return a, nil
	}

	// Forward everything else to the input field.
	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

// updateMenu handles key messages in the menu overlay.
func (a App) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.menu, cmd = a.menu.Update(msg)
	return a, cmd
}

// updateModePicker handles key messages in the mode picker overlay.
func (a App) updateModePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.modePicker, cmd = a.modePicker.Update(msg)
	return a, cmd
}

// updateHelp handles key messages in the help screen.
func (a App) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.help, cmd = a.help.Update(msg)
	return a, cmd
}

// updateTabEditor handles key messages in the tab editor.
func (a App) updateTabEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.tabEditor, cmd = a.tabEditor.Update(msg)
	return a, cmd
}

// updateHighlights handles key messages in the highlights manager.
func (a App) updateHighlights(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.highlightsMgr, cmd = a.highlightsMgr.Update(msg)
	return a, cmd
}

// updateWikiMenu handles key messages in the wiki menu overlay.
func (a App) updateWikiMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	var cmd tea.Cmd
	a.wikiMenu, cmd = a.wikiMenu.Update(msg)
	return a, cmd
}

// handleEvent routes EventMsg to the appropriate components.
func (a App) handleEvent(msg EventMsg) (tea.Model, tea.Cmd) {
	var routedTabs uint64
	for _, event := range msg.Events {
		switch ev := event.(type) {

		case types.GameTextEvent:
			// Apply color words first (if enabled), then highlights on top.
			styled := ev.Styled
			if a.colorWords {
				styled = colorwords.ApplyColorWords(styled)
			}
			styled = applyHighlights(styled, a.highlights)
			if a.hideIPs {
				styled = maskIPs(styled)
			}

			// Route text to All + matching custom tabs.
			routed := RouteText(a.tabs, styled, ev.Text, ev.IsEcho)
			// Only count non-empty text for unread markers.
			if ev.Text != "" {
				routedTabs |= routed
			}

		case types.SKOOTUpdateEvent:
			a.debug.UpdateSKOOT(ev)
			a.sidebar.UpdateVitals(ev.Health, ev.Fatigue, ev.Encumbrance, ev.Satiation)
			a.status.UpdateVitals(ev.Health, ev.Fatigue, ev.Encumbrance)
			if ev.Exits != nil {
				a.sidebar.UpdateExits(*ev.Exits)
			}
			if ev.Lighting != nil {
				a.sidebar.UpdateLighting(*ev.Lighting, ev.LightingRaw)
			}
			if len(ev.Rooms) > 0 || len(ev.Walls) > 0 {
				a.sidebar.UpdateMinimap(ev.Rooms, ev.Walls)
			}

		case types.MapURLEvent:
			a.sidebar.UpdateMapURL(ev.URL)

		case types.ModeChangeEvent:
			a.sidebar.UpdateMode(ev.NewMode)
			a.status.UpdateMode(ev.NewMode)

		case types.StatusUpdateEvent:
			a.sidebar.UpdateMode(ev.Mode)
			a.sidebar.UpdateDisplayState(ev.DisplayState)
			a.status.UpdateMode(ev.Mode)
			a.metrics.UpdateStatus(ev)
			a.metrics.UpdateMetrics(ev.MetricsCurrent, ev.MetricsHistory)

		case types.ConnectedEvent:
			a.status.SetConnected(true)

		case types.DisconnectedEvent:
			a.status.SetConnected(false)

		case types.ReconnectingEvent:
			a.status.SetReconnecting(ev.Attempt, ev.NextDelay)

		case types.WikiOpenMenuEvent:
			return a.Update(MenuWikiMsg{})
		}
	}

	// Mark unread only for tabs that actually received non-empty text.
	if routedTabs != 0 {
		for i := range a.tabs {
			if i != a.activeTab && routedTabs&(1<<uint(i)) != 0 {
				a.unread[i] = true
			}
		}
	}

	return a, nil
}

// ShowHelp opens the help screen.
func (a *App) ShowHelp() {
	a.help = NewHelpScreen()
	a.help.SetSize(a.width, a.height)
	a.state = stateHelp
}

// ShowModeError displays a mode-not-found error as a system message in the output.
// If any loaded modes are within edit distance 3 of the input, they are suggested.
func (a *App) ShowModeError(name string, modes []string) {
	var segments []types.StyledSegment
	segments = append(segments, types.StyledSegment{
		Text:  fmt.Sprintf("Mode %q not found.", name),
		Bold:  true,
		Color: "#ff5555",
	})

	// Find similar modes using Levenshtein distance.
	var similar []string
	for _, m := range modes {
		if textutil.Levenshtein(strings.ToLower(name), strings.ToLower(m)) <= 3 {
			similar = append(similar, m)
		}
	}
	if len(similar) > 0 {
		segments = append(segments, types.StyledSegment{
			Text:  " Did you mean: ",
			Color: "#e8a838",
		})
		for i, m := range similar {
			if i > 0 {
				segments = append(segments, types.StyledSegment{Text: ", "})
			}
			segments = append(segments, types.StyledSegment{
				Text:  m,
				Color: "#55cc55",
			})
		}
		segments = append(segments, types.StyledSegment{Text: "?"})
	}

	hr := []types.StyledSegment{{IsHR: true}}
	a.tabs[0].Pane.Append(hr)
	a.tabs[0].Pane.Append(segments)
	a.tabs[0].Pane.Append(hr)
}

// ShowModeList displays available modes as a system message in the output.
func (a *App) ShowModeList(modes []string) {
	var segments []types.StyledSegment
	segments = append(segments, types.StyledSegment{
		Text:  "Available modes: ",
		Bold:  true,
		Color: "#e8a838",
	})
	for i, mode := range modes {
		if i > 0 {
			segments = append(segments, types.StyledSegment{Text: ", "})
		}
		segments = append(segments, types.StyledSegment{
			Text:  mode,
			Color: "#55cc55",
		})
	}
	a.tabs[0].Pane.Append(segments)
}

// findTabByKind returns the index of the first tab with the given kind, or -1.
func (a App) findTabByKind(kind TabKind) int {
	for i, t := range a.tabs {
		if t.Kind == kind {
			return i
		}
	}
	return -1
}

// SwitchToDebug switches to the debug tab and enables debug mode.
func (a *App) SwitchToDebug() {
	a.debugMode = true
	idx := a.findTabByKind(TabKindDebug)
	if idx >= 0 {
		a.tabs[idx].Visible = true
		a.switchTab(idx)
	}
}

// switchTab switches to the given tab and clears its unread flag.
func (a *App) switchTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) {
		return
	}
	if !a.tabs[idx].Visible {
		return
	}
	a.activeTab = idx
	a.unread[idx] = false
}

// nextVisibleTab cycles to the next (dir=1) or previous (dir=-1) visible tab.
func (a *App) nextVisibleTab(dir int) {
	n := len(a.tabs)
	for i := 1; i < n; i++ {
		idx := (a.activeTab + i*dir + n) % n
		if a.tabs[idx].Visible {
			a.switchTab(idx)
			return
		}
	}
}

// switchToVisibleTab switches to the Nth visible tab (0-indexed).
func (a *App) switchToVisibleTab(n int) {
	count := 0
	for i, t := range a.tabs {
		if t.Visible {
			if count == n {
				a.switchTab(i)
				return
			}
			count++
		}
	}
}

// recalcLayout recalculates component sizes based on terminal dimensions.
func (a *App) recalcLayout() {
	// Compute effective sidebar visibility based on terminal size.
	// Minimap height + compass rows (7) = minimum for compact mode.
	// Compact + lighting (1) + vitals (4) = full mode.
	minimapAndCompass := a.sidebar.MinimapHeight() + compass.Rows
	fullSidebar := minimapAndCompass + 5 // lighting + 4 vitals

	a.sidebarVisible = a.sidebarOpen
	a.sidebarCompact = false

	if a.sidebarVisible {
		contentHeight := a.height - 6
		// Hide sidebar if width < 2x sidebar width.
		if a.width < a.sidebarWidth*2 {
			a.sidebarVisible = false
			// Hide sidebar if compass would be cut off.
		} else if contentHeight < minimapAndCompass+2 { // +2 for border
			a.sidebarVisible = false
			// Compact mode if vitals bars would be cut off.
		} else if contentHeight < fullSidebar+2 {
			a.sidebarCompact = true
		}
	}

	sidebarWidth := 0
	if a.sidebarVisible {
		sidebarWidth = a.sidebarWidth
	}

	// Heights: tabBar=1 + border=1, statusBar=1 + border=1, input=1 + border=1 = 6 total chrome
	contentHeight := a.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	contentWidth := a.width - sidebarWidth
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Update all tab panes
	for i := range a.tabs {
		a.tabs[i].Pane.SetSize(contentWidth, contentHeight)
	}
	a.metrics.SetSize(contentWidth, contentHeight)
	a.debug.SetSize(contentWidth, contentHeight)
	a.sidebar.SetSize(sidebarWidth, contentHeight)
	a.sidebar.SetCompact(a.sidebarCompact)
	a.status.SetWidth(a.width)
	a.input.SetWidth(a.width)
	a.login.SetSize(a.width, a.height)
	a.accountSelect.SetSize(a.width, a.height)
	a.splash.SetSize(a.width, a.height)
	a.credentialPrompt.SetSize(a.width, a.height)
	a.menu.SetSize(a.width, a.height)
	a.modePicker.SetSize(a.width, a.height)
	a.persistentData.SetSize(a.width, a.height)
	a.scriptDirsScreen.SetSize(a.width, a.height)
	a.priorityCmdsScreen.SetSize(a.width, a.height)
	a.notificationSettings.SetSize(a.width, a.height)
	a.wikiMenu.SetSize(a.width, a.height)
	a.highlightsMgr.SetSize(a.width, a.height)
	a.tabEditor.SetSize(a.width, a.height)
	a.help.SetSize(a.width, a.height)
}

// contentHeight returns the available content height.
func (a App) contentHeight() int {
	h := a.height - 6
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the entire TUI.
func (a App) View() string {
	switch a.state {
	case stateSplash:
		return a.splash.View()
	case stateAccountSelect:
		return a.accountSelect.View()
	case stateLogin:
		return a.login.View()
	case stateAuthenticating:
		return a.renderAuthenticating()
	case stateCredentialPrompt:
		return a.credentialPrompt.View()
	case stateMenu:
		return a.graphicsClear() + a.menu.View()
	case stateModePicker:
		return a.graphicsClear() + a.modePicker.View()
	case stateHighlights:
		return a.graphicsClear() + a.highlightsMgr.View()
	case stateHelp:
		return a.graphicsClear() + a.help.View()
	case stateTabEditor:
		return a.graphicsClear() + a.tabEditor.View()
	case statePersistentData:
		return a.graphicsClear() + a.persistentData.View()
	case stateScriptDirs:
		return a.graphicsClear() + a.scriptDirsScreen.View()
	case statePriorityCmds:
		return a.graphicsClear() + a.priorityCmdsScreen.View()
	case stateNotificationSettings:
		return a.graphicsClear() + a.notificationSettings.View()
	case stateWikiMenu:
		return a.graphicsClear() + a.wikiMenu.View()
	}

	// stateGame:

	var sections []string

	// Tab bar
	sections = append(sections, a.renderTabBar())

	// Content area (tab content + optional sidebar)
	var tabContent string
	if a.activeTab >= 0 && a.activeTab < len(a.tabs) {
		switch a.tabs[a.activeTab].Kind {
		case TabKindMetrics:
			tabContent = a.metrics.View()
		case TabKindDebug:
			tabContent = a.debug.View()
		default:
			tabContent = a.tabs[a.activeTab].Pane.View()
		}
	}

	// Fix the content pane width so the sidebar stays anchored to the right.
	sidebarWidth := 0
	if a.sidebarVisible {
		sidebarWidth = a.sidebarWidth
	}
	contentWidth := a.width - sidebarWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	// Pad each line of the tab content to exactly contentWidth so the sidebar
	// stays anchored to the right. We avoid lipgloss.Width() here because it
	// trims leading whitespace, which breaks indented game text.
	fixedContent := padLines(tabContent, contentWidth, a.contentHeight())

	var content string
	var kittyMinimap, kittyCompass string
	if a.sidebarVisible {
		sidebar := a.sidebar.View()
		kittyMinimap, kittyCompass = a.sidebar.GraphicsEscapes()
		content = lipgloss.JoinHorizontal(lipgloss.Top, fixedContent, sidebar)
	} else {
		content = fixedContent
	}

	sections = append(sections, content)

	// Status bar
	if a.pendingQuit {
		quitMsg := lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("Press Ctrl+C again to quit, or any key to cancel")
		sections = append(sections, statusBarStyle.Width(a.width).Render(quitMsg))
	} else {
		sections = append(sections, a.status.View())
	}

	// Input bar
	sections = append(sections, a.input.View())

	result := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Always clear previous Kitty images before drawing new ones.
	// This prevents ghost images after resize or tab switch.
	result += a.graphicsClear()

	// Inject Kitty graphics escapes AFTER layout, using ANSI cursor
	// positioning to place images in the sidebar.
	sidebarCol := a.width - a.sidebarWidth + 2
	if kittyMinimap != "" {
		// Minimap: row 2 (after tab bar)
		result += fmt.Sprintf("\033[s\033[%d;%dH%s\033[u", 2, sidebarCol, kittyMinimap)
	}
	if kittyCompass != "" {
		// Compass: after minimap (row 2 + minimap height)
		compassRow := 2 + a.sidebar.MinimapHeight()
		result += fmt.Sprintf("\033[s\033[%d;%dH%s\033[u", compassRow, sidebarCol, kittyCompass)
	}

	return result
}

// renderTabBar renders the tab bar with active/inactive styling and unread indicators.
func (a App) renderTabBar() string {
	var tabLabels []string

	for i, t := range a.tabs {
		if !t.Visible {
			continue
		}

		label := t.Name
		if i < len(a.unread) && a.unread[i] && i != a.activeTab {
			label = t.Name + " *"
		}

		if i == a.activeTab {
			tabLabels = append(tabLabels, activeTabStyle.Render(label))
		} else {
			tabLabels = append(tabLabels, inactiveTabStyle.Render(label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, tabLabels...)
	return tabBarStyle.Width(a.width).Render(bar)
}

// ipPattern matches IPv4 addresses in text.
var ipPattern = regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b`)

// ipMaskCache maps real IPs to their fake replacements for session consistency.
var ipMaskCache = make(map[string]string)

// maskIPs replaces IP addresses in styled segments with impossible IPs (octets 256-999).
// The same real IP always maps to the same fake IP within a session.
func maskIPs(segments []types.StyledSegment) []types.StyledSegment {
	var result []types.StyledSegment
	for _, seg := range segments {
		if seg.IsHR || seg.Text == "" {
			result = append(result, seg)
			continue
		}
		masked := ipPattern.ReplaceAllStringFunc(seg.Text, func(ip string) string {
			// Verify it's a real IP (all octets 0-255) before masking.
			parts := strings.Split(ip, ".")
			for _, p := range parts {
				n, _ := strconv.Atoi(p)
				if n > 255 {
					return ip // already impossible, leave it
				}
			}
			// Return cached fake IP if we've seen this one before.
			if fake, ok := ipMaskCache[ip]; ok {
				return fake
			}
			// Generate impossible IP: each octet 256-999.
			fake := strconv.Itoa(256+rand.Intn(744)) + "." +
				strconv.Itoa(256+rand.Intn(744)) + "." +
				strconv.Itoa(256+rand.Intn(744)) + "." +
				strconv.Itoa(256+rand.Intn(744))
			ipMaskCache[ip] = fake
			return fake
		})
		seg.Text = masked
		result = append(result, seg)
	}
	return result
}

// padLines pads or truncates each line of text to exactly width characters,
// and ensures exactly height lines of output. This preserves leading whitespace
// unlike lipgloss.Width() which trims it.
func padLines(text string, width, height int) string {
	lines := strings.Split(text, "\n")

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < len(lines) {
			line := lines[i]
			// Count visible length (strip ANSI escape sequences for measurement).
			visLen := lipgloss.Width(line)
			if visLen < width {
				b.WriteString(line)
				b.WriteString(strings.Repeat(" ", width-visLen))
			} else {
				b.WriteString(line)
			}
		} else {
			b.WriteString(strings.Repeat(" ", width))
		}
	}
	return b.String()
}

// renderAuthenticating shows a loading message during HTTP login.
func (a App) renderAuthenticating() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(40)

	content := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("Login to The Eternal City") +
		"\n\n" +
		lipgloss.NewStyle().Foreground(colorDim).Render("Authenticating...")

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, boxStyle.Render(content))
}
