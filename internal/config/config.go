package config

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/atomicfile"
	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/variables"
	"gopkg.in/yaml.v3"
)

// saveMu serializes Save so two overlapping writers (GUI setter goroutines,
// which Wails dispatches concurrently) can't interleave their file writes.
var saveMu sync.Mutex

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

type Config struct {
	Server            ServerConfig            `yaml:"server"`
	Commands          CommandsConfig          `yaml:"commands"`
	CharacterProfiles CharacterProfilesConfig `yaml:"character_profiles"`
	DisplayProfiles   DisplayProfilesConfig   `yaml:"display_profiles"`
	Credentials       CredentialsConfig       `yaml:"credentials"`
	Scripts           []string                `yaml:"scripts"`
	UI                UIConfig                `yaml:"ui"`
	Highlights        []HighlightConfig       `yaml:"highlights"`
	WoundHighlights   WoundHighlightConfig    `yaml:"wound_highlights"`
	SpeechColors      []SpeechColorConfig     `yaml:"speech_colors"`
	Kudos             KudosConfig             `yaml:"kudos"`
	Ignorelist        Ignorelist              `yaml:"ignorelist"`
	Notifications     NotificationsConfig     `yaml:"notifications"`
	Logging           LoggingConfig           `yaml:"logging"`
	Updates           UpdatesConfig           `yaml:"updates"`
	Play              PlayConfig              `yaml:"play"`
	Onboarding        OnboardingConfig        `yaml:"onboarding"`
}

// UpdatesConfig controls the GUI's startup check against GitHub releases.
type UpdatesConfig struct {
	Check bool `yaml:"check"` // notify when a newer release exists
}

// PlayConfig controls /play, the dramatic-presentation player.
type PlayConfig struct {
	// WaitForTimeout bounds every %wait-for that does not carry its own
	// override. Unbounded cue-waiting would strand the performer, since input is
	// locked down during playback.
	WaitForTimeout Duration `yaml:"wait_for_timeout"`
}

// CredentialsConfig selects the secure account credential backend. Backend is
// deliberately explicit: Praetor never falls back from a failed keyring to a
// plaintext or file-backed store.
type CredentialsConfig struct {
	Backend       string                         `yaml:"backend"`
	EncryptedFile EncryptedFileCredentialsConfig `yaml:"encrypted_file"`
}

// EncryptedFileCredentialsConfig controls the headless-service credential
// store. Path may be empty to use STATE_DIR/credentials/credentials.enc. KeyEnv
// names the environment variable containing a base64-encoded 32-byte key; the
// key itself is never written to config.yaml.
type EncryptedFileCredentialsConfig struct {
	Path   string `yaml:"path"`
	KeyEnv string `yaml:"key_env"`
}

// OnboardingConfig records one-time first-login experiences. It is persisted
// separately from user-facing UI preferences so reconnects and later launches
// cannot repeat them.
type OnboardingConfig struct {
	WelcomeShown bool `yaml:"welcome_shown"`
}

type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"`
	LoginURL string `yaml:"login_url"`
}

type CommandsConfig struct {
	DefaultDelay     Duration `yaml:"default_delay"`
	MinInterval      Duration `yaml:"min_interval"`
	MaxQueueSize     int      `yaml:"max_queue_size"`
	SemicolonDelayMS int      `yaml:"semicolon_delay_ms"`
	UnbusyDelayMS    int      `yaml:"unbusy_delay_ms"`
	HighPriority     []string `yaml:"high_priority"`
	// Variables are substituted in typed command-line input using ${name} or
	// ${name:fallback}.
	// They are deliberately separate from Lua mode state and never expand
	// recursively.
	// Variables is retained only to migrate pre-profile configuration. Runtime
	// code reads variables from the active CharacterProfileConfig.
	Variables map[string]string `yaml:"variables,omitempty" json:"-"`
}

const (
	defaultCommandDelay       = 1 * time.Second
	defaultCommandMinInterval = 500 * time.Millisecond
	DefaultSemicolonDelayMS   = 900
	MinSemicolonDelayMS       = 100
	MaxSemicolonDelayMS       = 10000
	DefaultUnbusyDelayMS      = 100
	MinUnbusyDelayMS          = 0
	MaxUnbusyDelayMS          = 10000
)

type HighlightConfig struct {
	Pattern string `yaml:"pattern"`
	Style   string `yaml:"style"` // red, gold, green, blue
	Active  bool   `yaml:"active"`
}

// WoundHighlightConfig controls opt-in styling for verified wound phrases in
// character descriptions. Each category selects one of the four color
// families; severity selects the shade within that family automatically.
type WoundHighlightConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Cuts      string `yaml:"cuts"`
	Bruises   string `yaml:"bruises"`
	Punctures string `yaml:"punctures"`
	Burns     string `yaml:"burns"`
}

// SpeechColorConfig assigns a Colorword style to recognized lines from a
// character pattern. Character expands input variables and supports * and ?
// wildcards. Ordinary speech is always eligible; think and OOC lines are
// opt-in. Style names come from colorwords.Catalog.
type SpeechColorConfig struct {
	Character    string `yaml:"character"`
	Style        string `yaml:"style"`
	Active       bool   `yaml:"active"`
	IncludeThink bool   `yaml:"include_think,omitempty"`
	IncludeOOC   bool   `yaml:"include_ooc,omitempty"`
}

// KudosConfig stores favorites and queued kudos messages, persisted in
// config.yaml. Favorites is a sorted list of character names. Queue is a
// FIFO of pending kudos messages.
type KudosConfig struct {
	Favorites []string          `yaml:"favorites"`
	Queue     []KudosQueueEntry `yaml:"queue"`
}

// KudosQueueEntry is a single queued kudos message.
type KudosQueueEntry struct {
	Name    string `yaml:"name"`
	Message string `yaml:"message"`
}

// HasFavorite reports whether name (case-insensitive, trim-insensitive)
// is already in the Favorites list.
func (k *KudosConfig) HasFavorite(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return false
	}
	for _, f := range k.Favorites {
		if strings.ToLower(f) == target {
			return true
		}
	}
	return false
}

// AddFavorite inserts name into Favorites if not already present
// (case-insensitive). The list is re-sorted case-insensitively.
// Returns true if the entry was newly added, false if it was a duplicate
// or empty after trimming.
func (k *KudosConfig) AddFavorite(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if k.HasFavorite(trimmed) {
		return false
	}
	k.Favorites = append(k.Favorites, trimmed)
	sort.SliceStable(k.Favorites, func(i, j int) bool {
		return strings.ToLower(k.Favorites[i]) < strings.ToLower(k.Favorites[j])
	})
	return true
}

// RemoveFavoriteAt removes the favorite at index i. Out-of-range is a no-op.
func (k *KudosConfig) RemoveFavoriteAt(i int) {
	if i < 0 || i >= len(k.Favorites) {
		return
	}
	k.Favorites = append(k.Favorites[:i], k.Favorites[i+1:]...)
}

// AddQueueEntry appends a new queue entry with name and message both trimmed.
// Empty (post-trim) name or message is a no-op.
func (k *KudosConfig) AddQueueEntry(name, message string) {
	n := strings.TrimSpace(name)
	m := strings.TrimSpace(message)
	if n == "" || m == "" {
		return
	}
	k.Queue = append(k.Queue, KudosQueueEntry{Name: n, Message: m})
}

// RemoveQueueAt removes the queue entry at index i. Out-of-range is a no-op.
func (k *KudosConfig) RemoveQueueAt(i int) {
	if i < 0 || i >= len(k.Queue) {
		return
	}
	k.Queue = append(k.Queue[:i], k.Queue[i+1:]...)
}

type Ignorelist struct {
	OOC   []string `yaml:"ooc"`
	Think []string `yaml:"think"`
}

type NotificationsConfig struct {
	Desktop DesktopNotificationsConfig `yaml:"desktop"`
}

type DesktopNotificationsConfig struct {
	AllowScriptNotifications bool                  `yaml:"allow_script_notifications"`
	Sound                    bool                  `yaml:"sound"`
	HealthBelow              ThresholdConfig       `yaml:"health_below"`
	FatigueBelow             ThresholdConfig       `yaml:"fatigue_below"`
	Patterns                 []NotifyPatternConfig `yaml:"patterns"`
}

type ThresholdConfig struct {
	Enabled   bool `yaml:"enabled"`
	Threshold int  `yaml:"threshold"`
}

type NotifyPatternConfig struct {
	Pattern string `yaml:"pattern"`
	Title   string `yaml:"title"`   // custom notification title
	Message string `yaml:"message"` // custom notification message
	Enabled bool   `yaml:"enabled"`
}

type LoggingConfig struct {
	App     AppLoggingConfig     `yaml:"app"`
	Session SessionLoggingConfig `yaml:"session"`
}

type AppLoggingConfig struct {
	Level     string `yaml:"level"` // debug, info, warn, error
	MaxSizeMB int    `yaml:"max_size_mb"`
	Retain    bool   `yaml:"retain"`
}

type SessionLoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // empty = XDG_CONFIG_HOME/praetor/logs/
}

type UIConfig struct {
	// DisplayMode controls how the minimap/compass/vitals are shown:
	//   "sidebar" — vertical strip down the right (default)
	//   "topbar"  — horizontal strip across the top
	//   "off"     — game pane only, sidebar/topbar hidden
	// Migrated from the legacy sidebar_open bool by migrateLegacyDisplay.
	DisplayMode      string   `yaml:"display_mode"`
	DefaultTab       string   `yaml:"default_tab"`
	Scrollback       int      `yaml:"scrollback"`
	SidebarWidth     int      `yaml:"sidebar_width"`
	GUISidebarWidth  int      `yaml:"gui_sidebar_width"` // pixels
	MinimapScale     float64  `yaml:"minimap_scale"`
	MinimapHeight    int      `yaml:"minimap_height"`
	GUIMinimapHeight int      `yaml:"gui_minimap_height"` // pixels
	CompassScale     float64  `yaml:"compass_scale"`
	OutputFontSize   int      `yaml:"output_font_size"`
	CRTScanlines     bool     `yaml:"crt_scanlines"`
	CRTRoll          bool     `yaml:"crt_roll"`
	CRTBloom         bool     `yaml:"crt_bloom"`
	QuickCycleModes  []string `yaml:"quick_cycle_modes"`
	ColorWords       bool     `yaml:"color_words"`
	EchoTyped        bool     `yaml:"echo_typed_commands"`
	EchoScript       bool     `yaml:"echo_script_commands"`
	HideIPs          bool     `yaml:"hide_ips"`
	// InputSpellcheck enables the webview's native spellchecker on the GUI
	// command input (red squiggles under misspelled words while composing says
	// and emotes). Engine support varies by platform webview.
	InputSpellcheck bool `yaml:"input_spellcheck"`
	// Mobile web presentation settings are persisted with the shared UI config,
	// but the native Wails and TUI shells deliberately ignore them.
	MobileOutputFontSize        int  `yaml:"mobile_output_font_size"`
	MobileShowToolbar           bool `yaml:"mobile_show_toolbar"`
	MobileShowTabBar            bool `yaml:"mobile_show_tab_bar"`
	MobileHideNavigationOnInput bool `yaml:"mobile_hide_navigation_on_input"`
	MobileLowercaseFirstLetter  bool `yaml:"mobile_lowercase_first_letter"`
	// KeepInputOnSend keeps the sent command in the GUI input, selected,
	// instead of clearing it: Enter re-sends it, and any typing or Backspace
	// replaces the whole line (native select-all semantics).
	KeepInputOnSend bool `yaml:"keep_input_on_send"`
	// NumpadNavigation controls the GUI numpad-walking behavior:
	//   "numlock" — move when NumLock is off; type digits when on (default)
	//   "always"  — numpad always sends movement (needed on macOS, which has
	//               no NumLock; the numpad can no longer type digits)
	//   "off"     — numpad navigation disabled
	NumpadNavigation string `yaml:"numpad_navigation"`
	// These fields are legacy migration inputs. Character-specific content now
	// lives in CharacterProfileConfig; keeping the fields omitempty lets old
	// configuration decode without writing duplicate canonical state.
	CustomTabs []CustomTabConfig `yaml:"custom_tabs,omitempty" json:"-"`
	ActionSets []ActionSet       `yaml:"action_sets,omitempty" json:"-"`
	// GUILayout is retained only to migrate the pre-display-profile placement
	// model. Runtime GUI/web code reads DisplayProfiles.
	GUILayout           GUILayoutConfig `yaml:"gui_layout,omitempty" json:"-"`
	SelectedActionSetID string          `yaml:"selected_action_set_id,omitempty" json:"-"`
}

type CustomTabConfig struct {
	ID           string          `yaml:"id"`
	Name         string          `yaml:"name"`
	Visible      bool            `yaml:"visible"`
	EchoCommands bool            `yaml:"echo_commands"` // only meaningful when tab is exclude-only
	Rules        []TabRuleConfig `yaml:"rules"`
}

// GUILayoutConfig is the unified placement model for GUI/web views. All is a
// fixed central tab and therefore deliberately absent from Views.
type GUILayoutConfig struct {
	SidebarsVisible bool               `yaml:"sidebars_visible"`
	Sidebars        []GUISidebarConfig `yaml:"sidebars"`
	Views           []ViewLayoutConfig `yaml:"views"`
}

type GUISidebarConfig struct {
	ID      int    `yaml:"id"`
	Enabled bool   `yaml:"enabled"`
	Side    string `yaml:"side"`
	Width   int    `yaml:"width"`
}

type ViewLayoutConfig struct {
	ID            string `yaml:"id"`
	Location      string `yaml:"location"`
	Priority      int    `yaml:"priority"`
	SidebarHeight int    `yaml:"sidebar_height"`
	Collapsed     bool   `yaml:"collapsed"`
}

const (
	DefaultDisplayProfileID = "0b40c7dd-fba7-44cb-83f7-f125f5bc98b3"
	DefaultSidebarOneID     = "5afa1264-4ce7-41d0-b7ea-2050e0d34b07"
	DefaultSidebarTwoID     = "026686ea-d46b-4d20-94f1-e1c2485d7e7b"
	DefaultSidebarThreeID   = "55148f61-39f1-4c16-be64-b9f3245e99bc"

	ComponentIDAll       = "a1ebe82e-7f0d-4ba9-9e00-181d045e3e12"
	ComponentIDMetrics   = "3888f7f2-16fa-41df-86bc-11c7a2488dbf"
	ComponentIDDebug     = "a1ba3fe5-10e2-406f-b1f1-7f8746c89762"
	ComponentIDMap       = "ccb0ec1f-9a08-4d4f-941e-2b8b2c6eb8f1"
	ComponentIDCompass   = "85178cda-7db8-4fe1-a09d-1a6e572016b2"
	ComponentIDVitals    = "e9a0240c-64df-4086-92aa-0f6372f8dde6"
	ComponentIDActions   = "fbf29dda-9c5c-4457-8a8b-d5a106206c29"
	ComponentIDModes     = "631a65d1-9261-4940-a18b-7ee7bfd48f12"
	ComponentIDVariables = "c043ea23-cb1d-4a30-97bd-01e588969d26"
)

// DisplayProfilesConfig is a registry from the outset. Character-profile
// association is added after the account/character selection prerequisite.
type DisplayProfilesConfig struct {
	DefaultID string                 `yaml:"default_id"`
	Profiles  []DisplayProfileConfig `yaml:"profiles"`
}

type DisplayProfileConfig struct {
	ID              string                   `yaml:"id"`
	Name            string                   `yaml:"name"`
	SidebarsVisible bool                     `yaml:"sidebars_visible"`
	Main            []DisplayComponentConfig `yaml:"main"`
	Sidebars        []DisplaySidebarConfig   `yaml:"sidebars"`
}

// DisplaySidebarConfig owns the ordered component list for one independently
// positioned column. UUID identity removes the former three-sidebar ceiling.
type DisplaySidebarConfig struct {
	ID         string                   `yaml:"id"`
	Name       string                   `yaml:"name"`
	Enabled    bool                     `yaml:"enabled"`
	Side       string                   `yaml:"side"`
	Width      int                      `yaml:"width"`
	Components []DisplayComponentConfig `yaml:"components"`
}

// DisplayComponentConfig places one UUID-addressed component. Built-ins use
// the fixed IDs above; Custom View IDs are generated UUIDs.
type DisplayComponentConfig struct {
	ComponentID   string `yaml:"component_id"`
	SidebarHeight int    `yaml:"sidebar_height,omitempty"`
	Collapsed     bool   `yaml:"collapsed,omitempty"`
}

type TabRuleConfig struct {
	Pattern string `yaml:"pattern"` // supports * and ? wildcards
	Include bool   `yaml:"include"` // true = "does match", false = "does not match"
	Active  bool   `yaml:"active"`
}

// ActionSet is a named group of quick-action buttons shown in the sidebar
// Actions tab. Users manage sets/buttons via the Action Sets editor.
type ActionSet struct {
	ID      string         `yaml:"id"`
	Name    string         `yaml:"name"`
	Buttons []ActionButton `yaml:"buttons"`
}

// ActionButton is a single quick-action: a label and the game command it sends.
type ActionButton struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
}

// DefaultCharacterProfileID is the immutable identity of the global template
// used whenever no game character is active.
const DefaultCharacterProfileID = "b7782d7d-5e5d-54d7-8c66-3e3c9a3b1f50"

// CharacterProfilesConfig stores the global Default plus account/character
// profiles selected from the game's play-number flow.
type CharacterProfilesConfig struct {
	DefaultID string                   `yaml:"default_id"`
	Profiles  []CharacterProfileConfig `yaml:"profiles"`
}

// CharacterProfileConfig owns settings whose values are expected to change
// with the played character. Display placement will reference this profile in
// a later slice; it intentionally remains outside this type for now.
type CharacterProfileConfig struct {
	ID                  string            `yaml:"id"`
	Name                string            `yaml:"name"`
	Account             string            `yaml:"account,omitempty"`
	CharacterNumber     int               `yaml:"character_number,omitempty"`
	Variables           map[string]string `yaml:"variables"`
	CustomViews         []CustomTabConfig `yaml:"custom_views"`
	ActionSets          []ActionSet       `yaml:"action_sets"`
	SelectedActionSetID string            `yaml:"selected_action_set_id,omitempty"`
}

// CharacterProfileIdentity is the editable identity projection used by the
// profile manager. Character-scoped content is intentionally absent so a
// rename or number change cannot overwrite variables, views, or action sets.
type CharacterProfileIdentity struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	Account         string `yaml:"account,omitempty"`
	CharacterNumber int    `yaml:"character_number,omitempty"`
}

// ValidUUID accepts the canonical lower/upper-case UUID text form. UUIDs are
// identifiers here rather than cryptographic secrets, so structural validation
// is sufficient and avoids another dependency.
func ValidUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func newUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		// crypto/rand failure is extraordinarily rare. Preserve the UUID shape
		// while incorporating time so validation can still repair the config.
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		copy(value[:], sum[:16])
	}
	return uuidFromBytes(value[:])
}

// NewUUID returns a fresh UUID suitable for persisted profile and component
// identities.
func NewUUID() string { return newUUID() }

func uuidFromBytes(source []byte) string {
	var value [16]byte
	copy(value[:], source)
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

// DefaultCharacterProfile returns the global default profile. Validate ensures
// it exists; nil only describes a manually constructed, unvalidated Config.
func (c *Config) DefaultCharacterProfile() *CharacterProfileConfig {
	for i := range c.CharacterProfiles.Profiles {
		if c.CharacterProfiles.Profiles[i].ID == c.CharacterProfiles.DefaultID {
			return &c.CharacterProfiles.Profiles[i]
		}
	}
	return nil
}

func (c *Config) DefaultDisplayProfile() *DisplayProfileConfig {
	for i := range c.DisplayProfiles.Profiles {
		if c.DisplayProfiles.Profiles[i].ID == c.DisplayProfiles.DefaultID {
			return &c.DisplayProfiles.Profiles[i]
		}
	}
	return nil
}

// PlaceUnplacedCustomViewsInDefault gives editors without placement controls
// (currently the TUI Custom Views editor) a safe default destination.
func (c *Config) PlaceUnplacedCustomViewsInDefault() {
	profile := c.DefaultDisplayProfile()
	character := c.DefaultCharacterProfile()
	if profile == nil || character == nil {
		return
	}
	placed := map[string]bool{}
	for _, component := range profile.Main {
		placed[component.ComponentID] = true
	}
	for _, sidebar := range profile.Sidebars {
		for _, component := range sidebar.Components {
			placed[component.ComponentID] = true
		}
	}
	for _, view := range character.CustomViews {
		if !placed[view.ID] {
			profile.Main = append(profile.Main, DisplayComponentConfig{ComponentID: view.ID})
		}
	}
}

func defaultDisplayProfile() DisplayProfileConfig {
	return DisplayProfileConfig{
		ID:              DefaultDisplayProfileID,
		Name:            "Default",
		SidebarsVisible: true,
		Main: []DisplayComponentConfig{
			{ComponentID: ComponentIDAll},
			{ComponentID: ComponentIDMetrics},
			{ComponentID: ComponentIDDebug},
		},
		Sidebars: []DisplaySidebarConfig{
			{
				ID: DefaultSidebarOneID, Name: "Sidebar 1", Enabled: true, Side: "right", Width: 260,
				Components: []DisplayComponentConfig{
					{ComponentID: ComponentIDMap, SidebarHeight: 160},
					{ComponentID: ComponentIDCompass, SidebarHeight: 220},
					{ComponentID: ComponentIDVitals, SidebarHeight: 140},
					{ComponentID: ComponentIDActions, SidebarHeight: 240},
					{ComponentID: ComponentIDModes, SidebarHeight: 240},
					{ComponentID: ComponentIDVariables, SidebarHeight: 260},
				},
			},
			{ID: DefaultSidebarTwoID, Name: "Sidebar 2", Side: "right", Width: 260, Components: []DisplayComponentConfig{}},
			{ID: DefaultSidebarThreeID, Name: "Sidebar 3", Side: "right", Width: 260, Components: []DisplayComponentConfig{}},
		},
	}
}

func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "game.eternalcitygame.com",
			Port:     8080,
			Protocol: "ws",
			LoginURL: "https://login.eternalcitygame.com/login.php",
		},
		Commands: CommandsConfig{
			DefaultDelay:     Duration{defaultCommandDelay},
			MinInterval:      Duration{defaultCommandMinInterval},
			MaxQueueSize:     20,
			SemicolonDelayMS: DefaultSemicolonDelayMS,
			UnbusyDelayMS:    DefaultUnbusyDelayMS,
			HighPriority:     []string{},
		},
		CharacterProfiles: CharacterProfilesConfig{
			DefaultID: DefaultCharacterProfileID,
			Profiles: []CharacterProfileConfig{{
				ID:          DefaultCharacterProfileID,
				Name:        "Default",
				Variables:   map[string]string{},
				CustomViews: []CustomTabConfig{},
				ActionSets:  []ActionSet{},
			}},
		},
		DisplayProfiles: DisplayProfilesConfig{
			DefaultID: DefaultDisplayProfileID,
			Profiles:  []DisplayProfileConfig{defaultDisplayProfile()},
		},
		Credentials: CredentialsConfig{
			Backend: "keyring",
			EncryptedFile: EncryptedFileCredentialsConfig{
				Path:   "",
				KeyEnv: "PRAETOR_CREDENTIALS_KEY",
			},
		},
		Scripts: []string{},
		UI: UIConfig{
			DisplayMode:          "sidebar",
			DefaultTab:           "all",
			Scrollback:           5000,
			SidebarWidth:         40,
			GUISidebarWidth:      260,
			MinimapScale:         1.0,
			MinimapHeight:        12,
			GUIMinimapHeight:     160,
			CompassScale:         1.0,
			OutputFontSize:       14,
			CRTScanlines:         true,
			CRTRoll:              true,
			CRTBloom:             true,
			QuickCycleModes:      []string{"disable"},
			EchoTyped:            true,
			EchoScript:           true,
			InputSpellcheck:      true,
			MobileOutputFontSize: 14,
			MobileShowToolbar:    true,
			MobileShowTabBar:     true,
			NumpadNavigation:     "numlock",
		},
		Highlights: []HighlightConfig{},
		WoundHighlights: WoundHighlightConfig{
			Cuts:      "red",
			Bruises:   "purple",
			Punctures: "blue",
			Burns:     "orange",
		},
		SpeechColors: []SpeechColorConfig{},
		Kudos: KudosConfig{
			Favorites: []string{},
			Queue:     []KudosQueueEntry{},
		},
		Notifications: NotificationsConfig{
			Desktop: DesktopNotificationsConfig{
				Sound: false,
				HealthBelow: ThresholdConfig{
					Enabled:   true,
					Threshold: 25,
				},
				FatigueBelow: ThresholdConfig{
					Enabled:   false,
					Threshold: 10,
				},
				Patterns: []NotifyPatternConfig{},
			},
		},
		Logging: LoggingConfig{
			App: AppLoggingConfig{
				Level:     "info",
				MaxSizeMB: 5,
				Retain:    false,
			},
			Session: SessionLoggingConfig{
				Enabled: true,
				Path:    "",
			},
		},
		Updates: UpdatesConfig{
			Check: true,
		},
		Play: PlayConfig{
			WaitForTimeout: Duration{60 * time.Second},
		},
		Onboarding: OnboardingConfig{
			WelcomeShown: false,
		},
	}
}

// Save writes the config to the given path atomically: it marshals, writes a
// temp file in the same directory, fsyncs it, then renames it over the target.
// A crash or power loss mid-write therefore leaves either the old complete file
// or the new one — never a truncated config.yaml that Load hard-fails on.
// Concurrent Saves are serialized. Multi-instance play is last-writer-wins, but
// the file is never torn.
func Save(cfg *Config, path string) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Preserve the existing file's permissions; default to 0644 for a new file.
	perm := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}

	return atomicfile.Write(path, data, perm)
}

func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	migrateLegacyEcho(cfg, data)
	migrateLegacyDisplay(cfg, data)
	migrateMobileOutputFontSize(cfg, data)
	migrateLegacyOnboarding(cfg, data)
	migrateLegacyCharacterProfiles(cfg, data)
	migrateLegacyDisplayProfiles(cfg, data)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// migrateLegacyCharacterProfiles moves the settings that predate character
// profiles into the one global Default profile. Presence of the new top-level
// section is authoritative, even when its collections are empty.
func migrateLegacyCharacterProfiles(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	if _, exists := raw["character_profiles"]; exists {
		return
	}
	variables := make(map[string]string, len(cfg.Commands.Variables))
	for name, value := range cfg.Commands.Variables {
		variables[name] = value
	}
	cfg.CharacterProfiles = CharacterProfilesConfig{
		DefaultID: DefaultCharacterProfileID,
		Profiles: []CharacterProfileConfig{{
			ID:                  DefaultCharacterProfileID,
			Name:                "Default",
			Variables:           variables,
			CustomViews:         append([]CustomTabConfig(nil), cfg.UI.CustomTabs...),
			ActionSets:          append([]ActionSet(nil), cfg.UI.ActionSets...),
			SelectedActionSetID: cfg.UI.SelectedActionSetID,
		}},
	}
	cfg.Commands.Variables = nil
	cfg.UI.CustomTabs = nil
	cfg.UI.ActionSets = nil
	cfg.UI.SelectedActionSetID = ""
}

// migrateLegacyDisplayProfiles converts the fixed three-sidebar/location model
// into a global UUID display profile. It intentionally leaves DisplayMode
// untouched because that field is still the TUI's sidebar/topbar preference.
func migrateLegacyDisplayProfiles(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	if _, exists := raw["display_profiles"]; exists {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	characterProfile := cfg.DefaultCharacterProfile()
	characterProfile.CustomViews = NormalizeCustomViewIDs(characterProfile.CustomViews)
	custom := characterProfile.CustomViews

	layout := cfg.UI.GUILayout
	if !ok {
		ui = map[string]interface{}{}
	}
	if _, exists := ui["gui_layout"]; !exists {
		layout = defaultGUILayout(custom, cfg.UI.GUISidebarWidth, cfg.UI.GUIMinimapHeight, cfg.UI.DisplayMode != "off")
	}

	// Old layout records refer to the pre-normalized Custom Tab ID. Match the
	// YAML list by index so those placements follow the migrated UUID.
	customIDs := map[string]string{}
	var legacy struct {
		UI struct {
			CustomTabs []CustomTabConfig `yaml:"custom_tabs"`
		} `yaml:"ui"`
	}
	if err := yaml.Unmarshal(data, &legacy); err == nil {
		for i, old := range legacy.UI.CustomTabs {
			if i < len(custom) && old.ID != "" {
				customIDs[old.ID] = custom[i].ID
			}
		}
	}
	cfg.DisplayProfiles = DisplayProfilesConfig{
		DefaultID: DefaultDisplayProfileID,
		Profiles:  []DisplayProfileConfig{displayProfileFromLegacy(layout, customIDs)},
	}
	cfg.UI.GUILayout = GUILayoutConfig{}
}

// migrateMobileOutputFontSize preserves the pre-split behavior for existing
// configurations. Before this field existed, mobile and desktop web output
// both used output_font_size; copy that value only when the mobile key is
// absent. Once saved, the two values remain independent.
func migrateMobileOutputFontSize(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	if _, exists := ui["mobile_output_font_size"]; !exists {
		// Match the validation that output_font_size received before the split,
		// including its fallback for legacy values below the desktop minimum.
		size := cfg.UI.OutputFontSize
		if size < 8 {
			size = 14
		} else if size > 40 {
			size = 40
		}
		cfg.UI.MobileOutputFontSize = size
	}
}

// migrateLegacyOnboarding prevents the first-login welcome popup from
// surprising existing installations after an upgrade. Fresh configs are saved
// with an explicit onboarding section and false marker; only configs predating
// the feature lack the section and are treated as already onboarded.
func migrateLegacyOnboarding(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	if _, ok := raw["onboarding"]; !ok {
		cfg.Onboarding.WelcomeShown = true
	}
}

// migrateLegacyEcho copies the deprecated ui.echo_commands value into the
// split EchoTyped/EchoScript fields when the new keys are absent.
func migrateLegacyEcho(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	legacy, hasLegacy := ui["echo_commands"]
	if !hasLegacy {
		return
	}
	b, ok := legacy.(bool)
	if !ok {
		return
	}
	if _, hasTyped := ui["echo_typed_commands"]; !hasTyped {
		cfg.UI.EchoTyped = b
	}
	if _, hasScript := ui["echo_script_commands"]; !hasScript {
		cfg.UI.EchoScript = b
	}
}

// migrateLegacyDisplay translates the deprecated ui.sidebar_open bool
// into the new ui.display_mode string when the new key is absent.
// sidebar_open=true  → display_mode="sidebar"
// sidebar_open=false → display_mode="off"
// Validate normalizes the result; unknown values fall back to "sidebar".
func migrateLegacyDisplay(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	if _, hasNew := ui["display_mode"]; hasNew {
		return
	}
	legacy, hasLegacy := ui["sidebar_open"]
	if !hasLegacy {
		return
	}
	b, ok := legacy.(bool)
	if !ok {
		return
	}
	if b {
		cfg.UI.DisplayMode = "sidebar"
	} else {
		cfg.UI.DisplayMode = "off"
	}
}

// Validate checks the config for invalid or out-of-range values,
// clamping to sensible defaults where possible and returning an
// error only for truly broken configuration.
// TransportWarnings returns non-fatal advisories about cleartext transport: an
// http:// login URL POSTs the raw password unencrypted, and the ws:// protocol
// sends the credential-equivalent cookies, the MD5 handshake, and all game
// traffic in the clear. These are warnings, not validation errors — the shipped
// default is ws:// because the game server may not offer TLS, so making it an
// error would brick the default config. The shells log these at startup.
func (c *Config) TransportWarnings() []string {
	var warnings []string
	if strings.HasPrefix(strings.ToLower(c.Server.LoginURL), "http://") {
		warnings = append(warnings, fmt.Sprintf(
			"server.login_url uses cleartext http:// (%s) — your password is sent unencrypted; use https:// if the server supports it",
			c.Server.LoginURL))
	}
	if c.Server.Protocol == "ws" {
		warnings = append(warnings,
			"server.protocol is ws:// (cleartext) — session cookies and game traffic are unencrypted; use wss:// if the server supports it")
	}
	return warnings
}

func (c *Config) Validate() error {
	// Server
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	if c.Server.Protocol != "ws" && c.Server.Protocol != "wss" {
		return fmt.Errorf("server.protocol must be 'ws' or 'wss', got %q", c.Server.Protocol)
	}
	if c.Server.LoginURL == "" {
		return fmt.Errorf("server.login_url is required")
	}

	c.normalizeCharacterProfiles()

	// Commands
	if c.Commands.DefaultDelay.Duration < 100*time.Millisecond {
		c.Commands.DefaultDelay = Duration{defaultCommandDelay}
	}
	if c.Commands.MinInterval.Duration < 50*time.Millisecond {
		c.Commands.MinInterval = Duration{defaultCommandMinInterval}
	}
	if c.Commands.MaxQueueSize < 1 {
		c.Commands.MaxQueueSize = 20
	}
	if c.Commands.SemicolonDelayMS < MinSemicolonDelayMS || c.Commands.SemicolonDelayMS > MaxSemicolonDelayMS {
		c.Commands.SemicolonDelayMS = DefaultSemicolonDelayMS
	}
	if c.Commands.UnbusyDelayMS < MinUnbusyDelayMS || c.Commands.UnbusyDelayMS > MaxUnbusyDelayMS {
		c.Commands.UnbusyDelayMS = DefaultUnbusyDelayMS
	}

	// Play
	if c.Play.WaitForTimeout.Duration <= 0 {
		c.Play.WaitForTimeout = Duration{60 * time.Second}
	}

	// Credentials. Backends are never inferred from runtime availability: a
	// missing keyring remains a visible keyring error, and an encrypted file
	// requires its own independently supplied key.
	switch c.Credentials.Backend {
	case "keyring", "encrypted_file", "disabled":
	default:
		return fmt.Errorf("credentials.backend must be 'keyring', 'encrypted_file', or 'disabled', got %q", c.Credentials.Backend)
	}
	if c.Credentials.EncryptedFile.KeyEnv == "" {
		c.Credentials.EncryptedFile.KeyEnv = "PRAETOR_CREDENTIALS_KEY"
	}
	for i, r := range c.Credentials.EncryptedFile.KeyEnv {
		if (i == 0 && r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')) ||
			(i > 0 && r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9')) {
			return fmt.Errorf("credentials.encrypted_file.key_env is not a valid environment variable name")
		}
	}

	// UI
	validDisplayModes := map[string]bool{"sidebar": true, "topbar": true, "off": true}
	if !validDisplayModes[c.UI.DisplayMode] {
		c.UI.DisplayMode = "sidebar"
	}
	validTabs := map[string]bool{"all": true, "general": true, "combat": true, "social": true, "metrics": true}
	if !validTabs[c.UI.DefaultTab] {
		c.UI.DefaultTab = "all"
	}
	if c.UI.Scrollback < 100 {
		c.UI.Scrollback = 5000
	}
	if c.UI.SidebarWidth < 20 {
		c.UI.SidebarWidth = 40
	}
	if c.UI.GUISidebarWidth < 180 {
		c.UI.GUISidebarWidth = 260
	} else if c.UI.GUISidebarWidth > 600 {
		c.UI.GUISidebarWidth = 600
	}
	if c.UI.MinimapScale <= 0 {
		c.UI.MinimapScale = 1.0
	}
	if c.UI.MinimapHeight < 4 {
		c.UI.MinimapHeight = 12
	}
	if c.UI.GUIMinimapHeight < 80 {
		c.UI.GUIMinimapHeight = 160
	} else if c.UI.GUIMinimapHeight > 400 {
		c.UI.GUIMinimapHeight = 400
	}
	if c.UI.CompassScale <= 0 {
		c.UI.CompassScale = 1.0
	}
	if c.UI.OutputFontSize < 8 {
		c.UI.OutputFontSize = 14
	} else if c.UI.OutputFontSize > 40 {
		c.UI.OutputFontSize = 40
	}
	if c.UI.MobileOutputFontSize < 6 {
		c.UI.MobileOutputFontSize = 6
	} else if c.UI.MobileOutputFontSize > 40 {
		c.UI.MobileOutputFontSize = 40
	}
	if len(c.UI.QuickCycleModes) == 0 {
		c.UI.QuickCycleModes = []string{"disable"}
	}
	// Numpad navigation mode: fall back to "numlock" for empty (pre-existing
	// configs) or unrecognized values.
	switch c.UI.NumpadNavigation {
	case "numlock", "always", "off":
	default:
		c.UI.NumpadNavigation = "numlock"
	}
	allCustomViews := make([]CustomTabConfig, 0)
	for _, profile := range c.CharacterProfiles.Profiles {
		allCustomViews = append(allCustomViews, profile.CustomViews...)
	}
	c.normalizeDisplayProfiles(allCustomViews)
	// Clear migration-only aliases so Save writes one canonical copy.
	c.Commands.Variables = nil
	c.UI.CustomTabs = nil
	c.UI.ActionSets = nil
	c.UI.SelectedActionSetID = ""
	c.UI.GUILayout = GUILayoutConfig{}

	// Highlights
	validStyles := map[string]bool{"red": true, "gold": true, "green": true, "blue": true}
	kept := c.Highlights[:0]
	for i := range c.Highlights {
		// Drop empty/whitespace-only patterns: a case-insensitive empty-substring
		// match highlights every line. Reachable only by hand-editing config.yaml.
		if strings.TrimSpace(c.Highlights[i].Pattern) == "" {
			log.Printf("[CONFIG] dropping highlight with empty pattern")
			continue
		}
		if !validStyles[c.Highlights[i].Style] {
			c.Highlights[i].Style = "gold"
		}
		kept = append(kept, c.Highlights[i])
	}
	c.Highlights = kept

	// Wound highlighting is opt-in. Empty family values occur naturally when
	// loading a pre-feature config, so repair each independently to its category
	// default without changing Enabled.
	validWoundFamilies := map[string]bool{"red": true, "purple": true, "blue": true, "orange": true}
	if !validWoundFamilies[c.WoundHighlights.Cuts] {
		c.WoundHighlights.Cuts = "red"
	}
	if !validWoundFamilies[c.WoundHighlights.Bruises] {
		c.WoundHighlights.Bruises = "purple"
	}
	if !validWoundFamilies[c.WoundHighlights.Punctures] {
		c.WoundHighlights.Punctures = "blue"
	}
	if !validWoundFamilies[c.WoundHighlights.Burns] {
		c.WoundHighlights.Burns = "orange"
	}

	// Speech colors
	seenCharacters := make(map[string]bool, len(c.SpeechColors))
	keptSpeechColors := c.SpeechColors[:0]
	for _, rule := range c.SpeechColors {
		rule.Character = strings.TrimSpace(rule.Character)
		if rule.Character == "" {
			log.Printf("[CONFIG] dropping speech color with empty character pattern")
			continue
		}
		style, ok := colorwords.LookupStyle(rule.Style)
		if !ok {
			log.Printf("[CONFIG] dropping speech color for %q with unknown style %q", rule.Character, rule.Style)
			continue
		}
		key := strings.ToLower(rule.Character)
		if seenCharacters[key] {
			log.Printf("[CONFIG] dropping duplicate speech color character pattern %q", rule.Character)
			continue
		}
		seenCharacters[key] = true
		rule.Style = style.Name
		keptSpeechColors = append(keptSpeechColors, rule)
	}
	c.SpeechColors = keptSpeechColors

	// Notifications
	if c.Notifications.Desktop.HealthBelow.Threshold < 0 || c.Notifications.Desktop.HealthBelow.Threshold > 100 {
		c.Notifications.Desktop.HealthBelow.Threshold = 25
	}
	if c.Notifications.Desktop.FatigueBelow.Threshold < 0 || c.Notifications.Desktop.FatigueBelow.Threshold > 100 {
		c.Notifications.Desktop.FatigueBelow.Threshold = 10
	}

	// Logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.App.Level] {
		c.Logging.App.Level = "info"
	}
	if c.Logging.App.MaxSizeMB < 1 {
		c.Logging.App.MaxSizeMB = 5
	}

	return nil
}

func (c *Config) normalizeCharacterProfiles() {
	profiles := c.CharacterProfiles.Profiles
	defaultIndex := -1
	if ValidUUID(c.CharacterProfiles.DefaultID) {
		for i := range profiles {
			if profiles[i].ID == c.CharacterProfiles.DefaultID {
				defaultIndex = i
				break
			}
		}
	}
	if defaultIndex < 0 {
		if len(profiles) == 0 {
			profiles = append(profiles, CharacterProfileConfig{})
		}
		defaultIndex = 0
		profiles[defaultIndex].ID = DefaultCharacterProfileID
		c.CharacterProfiles.DefaultID = DefaultCharacterProfileID
	}

	seen := make(map[string]bool, len(profiles))
	seenComponentIDs := make(map[string]bool)
	for i := range profiles {
		profile := &profiles[i]
		if i == defaultIndex {
			if !ValidUUID(profile.ID) {
				profile.ID = DefaultCharacterProfileID
				c.CharacterProfiles.DefaultID = profile.ID
			}
			if strings.TrimSpace(profile.Name) == "" {
				profile.Name = "Default"
			}
			profile.Account = ""
			profile.CharacterNumber = 0
		} else {
			profile.Name = strings.TrimSpace(profile.Name)
			if profile.Name == "" {
				profile.Name = "Profile"
			}
			if !ValidUUID(profile.ID) || seen[strings.ToLower(profile.ID)] {
				profile.ID = newUUID()
			}
			profile.Account = strings.TrimSpace(profile.Account)
			if profile.CharacterNumber < 0 {
				profile.CharacterNumber = 0
			}
		}
		seen[strings.ToLower(profile.ID)] = true
		if profile.Variables == nil {
			profile.Variables = map[string]string{}
		}
		for name := range profile.Variables {
			if !ValidVariableName(name) {
				log.Printf("[CONFIG] dropping command variable with invalid name %q from profile %q", name, profile.Name)
				delete(profile.Variables, name)
			}
		}
		profile.CustomViews = NormalizeCustomViewIDs(profile.CustomViews)
		for viewIndex := range profile.CustomViews {
			key := strings.ToLower(profile.CustomViews[viewIndex].ID)
			if seenComponentIDs[key] {
				profile.CustomViews[viewIndex].ID = newUUID()
				key = strings.ToLower(profile.CustomViews[viewIndex].ID)
			}
			seenComponentIDs[key] = true
		}
		if profile.CustomViews == nil {
			profile.CustomViews = []CustomTabConfig{}
		}
		profile.ActionSets = NormalizeActionSetIDs(profile.ActionSets)
		if profile.ActionSets == nil {
			profile.ActionSets = []ActionSet{}
		}
		profile.SelectedActionSetID = normalizeSelectedActionSetID(profile.SelectedActionSetID, profile.ActionSets)
	}
	c.CharacterProfiles.Profiles = profiles
}

func (c *Config) normalizeDisplayProfiles(custom []CustomTabConfig) {
	profiles := c.DisplayProfiles.Profiles
	defaultIndex := -1
	if ValidUUID(c.DisplayProfiles.DefaultID) {
		for i := range profiles {
			if profiles[i].ID == c.DisplayProfiles.DefaultID {
				defaultIndex = i
				break
			}
		}
	}
	if defaultIndex < 0 {
		if len(profiles) == 0 {
			profiles = append(profiles, defaultDisplayProfile())
		}
		defaultIndex = 0
		profiles[0].ID = DefaultDisplayProfileID
		c.DisplayProfiles.DefaultID = DefaultDisplayProfileID
	}

	allowed := make(map[string]bool, len(builtinComponentKinds)+len(custom))
	for id := range builtinComponentKinds {
		allowed[id] = true
	}
	for _, view := range custom {
		allowed[view.ID] = true
	}
	seenProfiles := map[string]bool{}
	for i := range profiles {
		profile := &profiles[i]
		if i == defaultIndex {
			if !ValidUUID(profile.ID) {
				profile.ID = DefaultDisplayProfileID
				c.DisplayProfiles.DefaultID = profile.ID
			}
			if strings.TrimSpace(profile.Name) == "" {
				profile.Name = "Default"
			}
		} else {
			profile.Name = strings.TrimSpace(profile.Name)
			if profile.Name == "" {
				profile.Name = "Display Profile"
			}
			if !ValidUUID(profile.ID) || seenProfiles[strings.ToLower(profile.ID)] {
				profile.ID = newUUID()
			}
		}
		seenProfiles[strings.ToLower(profile.ID)] = true

		seenComponents := map[string]bool{}
		profile.Main = normalizeDisplayComponents(profile.Main, allowed, seenComponents, false)
		if !seenComponents[ComponentIDAll] {
			profile.Main = append([]DisplayComponentConfig{{ComponentID: ComponentIDAll}}, profile.Main...)
			seenComponents[ComponentIDAll] = true
		} else {
			for i, component := range profile.Main {
				if component.ComponentID == ComponentIDAll && i != 0 {
					profile.Main = append([]DisplayComponentConfig{component}, append(profile.Main[:i], profile.Main[i+1:]...)...)
					break
				}
			}
		}

		seenSidebars := map[string]bool{}
		for j := range profile.Sidebars {
			sidebar := &profile.Sidebars[j]
			if !ValidUUID(sidebar.ID) || seenSidebars[strings.ToLower(sidebar.ID)] {
				sidebar.ID = newUUID()
			}
			seenSidebars[strings.ToLower(sidebar.ID)] = true
			sidebar.Name = strings.TrimSpace(sidebar.Name)
			if sidebar.Name == "" {
				sidebar.Name = fmt.Sprintf("Sidebar %d", j+1)
			}
			if sidebar.Side != "left" && sidebar.Side != "right" {
				sidebar.Side = "right"
			}
			if sidebar.Width < 180 {
				sidebar.Width = 260
			} else if sidebar.Width > 600 {
				sidebar.Width = 600
			}
			sidebar.Components = normalizeDisplayComponents(sidebar.Components, allowed, seenComponents, true)
			if sidebar.Components == nil {
				sidebar.Components = []DisplayComponentConfig{}
			}
		}
		if profile.Main == nil {
			profile.Main = []DisplayComponentConfig{{ComponentID: ComponentIDAll}}
		}
		if profile.Sidebars == nil {
			profile.Sidebars = []DisplaySidebarConfig{}
		}
	}
	c.DisplayProfiles.Profiles = profiles
}

func normalizeDisplayComponents(components []DisplayComponentConfig, allowed, seen map[string]bool, sidebar bool) []DisplayComponentConfig {
	result := make([]DisplayComponentConfig, 0, len(components))
	for _, component := range components {
		if !allowed[component.ComponentID] || seen[component.ComponentID] {
			continue
		}
		seen[component.ComponentID] = true
		if sidebar {
			if component.SidebarHeight < 80 || component.SidebarHeight > 800 {
				component.SidebarHeight = defaultComponentHeight(component.ComponentID)
			}
		} else {
			component.SidebarHeight = 0
			component.Collapsed = false
		}
		result = append(result, component)
	}
	return result
}

// NormalizeActionSetIDs returns a shallow copy that preserves the first
// occurrence of every non-empty ID and deterministically repairs legacy,
// missing, or duplicate IDs. The deterministic fallback keeps a legacy config
// stable across repeated loads even before the application next saves the
// migrated representation.
func NormalizeActionSetIDs(sets []ActionSet) []ActionSet {
	normalized := append([]ActionSet(nil), sets...)
	seen := make(map[string]bool, len(normalized))
	for i := range normalized {
		id := strings.TrimSpace(normalized[i].ID)
		if id != "" && !seen[id] {
			normalized[i].ID = id
			seen[id] = true
			continue
		}
		for attempt := 0; ; attempt++ {
			id = actionSetMigrationID(normalized[i], i, attempt)
			if !seen[id] {
				normalized[i].ID = id
				seen[id] = true
				break
			}
		}
	}
	return normalized
}

func actionSetMigrationID(set ActionSet, index, attempt int) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "praetor-action-set\x00%d\x00%d\x00%s\x00", index, attempt, set.Name)
	for _, button := range set.Buttons {
		fmt.Fprintf(hash, "%s\x00%s\x00", button.Label, button.Command)
	}
	return fmt.Sprintf("action-%x", hash.Sum(nil)[:12])
}

var builtinViewDefaults = []ViewLayoutConfig{
	{ID: "metrics", Location: "tab", Priority: 100, SidebarHeight: 260},
	{ID: "debug", Location: "tab", Priority: 0, SidebarHeight: 300},
	{ID: "map", Location: "sidebar-1", Priority: 900, SidebarHeight: 160},
	{ID: "exits", Location: "sidebar-1", Priority: 800, SidebarHeight: 220},
	{ID: "vitals", Location: "sidebar-1", Priority: 700, SidebarHeight: 140},
	{ID: "actions", Location: "sidebar-1", Priority: 600, SidebarHeight: 240},
	{ID: "modes", Location: "sidebar-1", Priority: 500, SidebarHeight: 240},
	{ID: "variables", Location: "sidebar-1", Priority: 400, SidebarHeight: 260},
}

var builtinComponentKinds = map[string]string{
	ComponentIDAll:       "all",
	ComponentIDMetrics:   "metrics",
	ComponentIDDebug:     "debug",
	ComponentIDMap:       "map",
	ComponentIDCompass:   "exits",
	ComponentIDVitals:    "vitals",
	ComponentIDActions:   "actions",
	ComponentIDModes:     "modes",
	ComponentIDVariables: "variables",
}

// BuiltinComponentKinds returns the fixed UUID-to-renderer registry. Returning
// a copy prevents callers from mutating configuration identity.
func BuiltinComponentKinds() map[string]string {
	result := make(map[string]string, len(builtinComponentKinds))
	for id, kind := range builtinComponentKinds {
		result[id] = kind
	}
	return result
}

func legacyComponentID(id string, customIDs map[string]string) string {
	for componentID, kind := range builtinComponentKinds {
		if kind == id {
			return componentID
		}
	}
	if migrated := customIDs[id]; migrated != "" {
		return migrated
	}
	if ValidUUID(id) {
		return id
	}
	return ""
}

func defaultComponentHeight(id string) int {
	switch id {
	case ComponentIDMap:
		return 160
	case ComponentIDCompass:
		return 220
	case ComponentIDVitals:
		return 140
	case ComponentIDMetrics, ComponentIDVariables:
		return 260
	case ComponentIDDebug:
		return 300
	case ComponentIDActions, ComponentIDModes:
		return 240
	default:
		return 300
	}
}

func displayProfileFromLegacy(layout GUILayoutConfig, customIDs map[string]string) DisplayProfileConfig {
	profile := DisplayProfileConfig{
		ID:              DefaultDisplayProfileID,
		Name:            "Default",
		SidebarsVisible: layout.SidebarsVisible,
		Main:            []DisplayComponentConfig{{ComponentID: ComponentIDAll}},
		Sidebars:        make([]DisplaySidebarConfig, 0, len(layout.Sidebars)),
	}
	views := append([]ViewLayoutConfig(nil), layout.Views...)
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].Location != views[j].Location {
			return views[i].Location < views[j].Location
		}
		return views[i].Priority > views[j].Priority
	})
	for _, view := range views {
		if view.Location != "tab" {
			continue
		}
		if id := legacyComponentID(view.ID, customIDs); id != "" && id != ComponentIDAll {
			profile.Main = append(profile.Main, DisplayComponentConfig{ComponentID: id})
		}
	}
	for i, old := range layout.Sidebars {
		sidebarID := newUUID()
		switch i {
		case 0:
			sidebarID = DefaultSidebarOneID
		case 1:
			sidebarID = DefaultSidebarTwoID
		case 2:
			sidebarID = DefaultSidebarThreeID
		}
		sidebar := DisplaySidebarConfig{
			ID: sidebarID, Name: fmt.Sprintf("Sidebar %d", i+1), Enabled: old.Enabled,
			Side: old.Side, Width: old.Width, Components: []DisplayComponentConfig{},
		}
		location := fmt.Sprintf("sidebar-%d", old.ID)
		for _, view := range views {
			if view.Location != location {
				continue
			}
			if id := legacyComponentID(view.ID, customIDs); id != "" {
				height := view.SidebarHeight
				if height == 0 {
					height = defaultComponentHeight(id)
				}
				sidebar.Components = append(sidebar.Components, DisplayComponentConfig{
					ComponentID: id, SidebarHeight: height, Collapsed: view.Collapsed,
				})
			}
		}
		profile.Sidebars = append(profile.Sidebars, sidebar)
	}
	return profile
}

// FindDisplayComponent returns a copy of a component placement from main or a
// sidebar. Its container and order remain owned by the display profile.
func FindDisplayComponent(profile DisplayProfileConfig, id string) (DisplayComponentConfig, bool) {
	for _, component := range profile.Main {
		if component.ComponentID == id {
			return component, true
		}
	}
	for _, sidebar := range profile.Sidebars {
		for _, component := range sidebar.Components {
			if component.ComponentID == id {
				return component, true
			}
		}
	}
	return DisplayComponentConfig{}, false
}

// BuiltinViewIDs returns the stable registry IDs owned by the GUI/web shell.
func BuiltinViewIDs() []string {
	ids := make([]string, len(builtinViewDefaults))
	for i, view := range builtinViewDefaults {
		ids[i] = view.ID
	}
	return ids
}

func defaultGUILayout(custom []CustomTabConfig, legacyWidth, legacyMapHeight int, visible bool) GUILayoutConfig {
	if legacyWidth < 180 {
		legacyWidth = 260
	} else if legacyWidth > 600 {
		legacyWidth = 600
	}
	if legacyMapHeight < 80 {
		legacyMapHeight = 160
	} else if legacyMapHeight > 800 {
		legacyMapHeight = 800
	}
	views := append([]ViewLayoutConfig(nil), builtinViewDefaults...)
	for i := range views {
		if views[i].ID == "map" {
			views[i].SidebarHeight = legacyMapHeight
		}
	}
	for i, tab := range custom {
		views = append(views, ViewLayoutConfig{
			ID: tab.ID, Location: "tab", Priority: 1000 - i,
			SidebarHeight: 300,
		})
	}
	return GUILayoutConfig{
		SidebarsVisible: visible,
		Sidebars: []GUISidebarConfig{
			{ID: 1, Enabled: true, Side: "right", Width: legacyWidth},
			{ID: 2, Enabled: false, Side: "right", Width: 260},
			{ID: 3, Enabled: false, Side: "right", Width: 260},
		},
		Views: views,
	}
}

// NormalizeCustomViewIDs gives legacy Custom Tabs stable identities so their
// layout and runtime buffers survive rename and reorder.
func NormalizeCustomViewIDs(tabs []CustomTabConfig) []CustomTabConfig {
	normalized := append([]CustomTabConfig(nil), tabs...)
	seen := make(map[string]bool, len(normalized))
	for i := range normalized {
		id := strings.TrimSpace(normalized[i].ID)
		if ValidUUID(id) && !seen[strings.ToLower(id)] && builtinComponentKinds[id] == "" {
			normalized[i].ID = id
			seen[strings.ToLower(id)] = true
			continue
		}
		for attempt := 0; ; attempt++ {
			hash := sha256.New()
			fmt.Fprintf(hash, "praetor-custom-view\x00%d\x00%d\x00%s\x00%t\x00%t\x00", i, attempt, normalized[i].Name, normalized[i].Visible, normalized[i].EchoCommands)
			for _, rule := range normalized[i].Rules {
				fmt.Fprintf(hash, "%s\x00%t\x00%t\x00", rule.Pattern, rule.Include, rule.Active)
			}
			id = uuidFromBytes(hash.Sum(nil))
			if !seen[strings.ToLower(id)] && builtinComponentKinds[id] == "" {
				normalized[i].ID = id
				seen[strings.ToLower(id)] = true
				break
			}
		}
	}
	return normalized
}

// NormalizeGUILayout repairs malformed placement data without losing Custom
// View definitions. The first occurrence of a view wins; missing registered
// views are restored at deterministic defaults.
func NormalizeGUILayout(layout GUILayoutConfig, custom []CustomTabConfig) GUILayoutConfig {
	defaults := defaultGUILayout(custom, 260, 160, true)

	sidebars := make([]GUISidebarConfig, 3)
	seenSidebars := map[int]bool{}
	for _, sidebar := range layout.Sidebars {
		if sidebar.ID < 1 || sidebar.ID > 3 || seenSidebars[sidebar.ID] {
			continue
		}
		seenSidebars[sidebar.ID] = true
		if sidebar.Side != "left" && sidebar.Side != "right" {
			sidebar.Side = "right"
		}
		if sidebar.Width < 180 {
			sidebar.Width = 260
		} else if sidebar.Width > 600 {
			sidebar.Width = 600
		}
		sidebars[sidebar.ID-1] = sidebar
	}
	for i := range sidebars {
		if !seenSidebars[i+1] {
			sidebars[i] = defaults.Sidebars[i]
		}
	}
	layout.Sidebars = sidebars

	allowed := make(map[string]ViewLayoutConfig, len(defaults.Views))
	for _, view := range defaults.Views {
		allowed[view.ID] = view
	}
	seenViews := map[string]bool{}
	views := make([]ViewLayoutConfig, 0, len(allowed))
	for _, view := range layout.Views {
		fallback, ok := allowed[view.ID]
		if !ok || seenViews[view.ID] {
			continue
		}
		seenViews[view.ID] = true
		if !validViewLocation(view.Location) {
			view.Location = fallback.Location
		}
		if view.SidebarHeight < 80 {
			view.SidebarHeight = fallback.SidebarHeight
		} else if view.SidebarHeight > 800 {
			view.SidebarHeight = 800
		}
		views = append(views, view)
	}
	for _, fallback := range defaults.Views {
		if !seenViews[fallback.ID] {
			views = append(views, fallback)
		}
	}
	layout.Views = views
	return layout
}

func validViewLocation(location string) bool {
	switch location {
	case "tab", "sidebar-1", "sidebar-2", "sidebar-3":
		return true
	default:
		return false
	}
}

func isBuiltinViewID(id string) bool {
	for _, view := range builtinViewDefaults {
		if view.ID == id {
			return true
		}
	}
	return false
}

// FindViewLayout looks up a placement by stable view ID.
func FindViewLayout(layout GUILayoutConfig, id string) (ViewLayoutConfig, bool) {
	for _, view := range layout.Views {
		if view.ID == id {
			return view, true
		}
	}
	return ViewLayoutConfig{}, false
}

func normalizeSelectedActionSetID(selected string, sets []ActionSet) string {
	for _, set := range sets {
		if set.ID == selected {
			return selected
		}
	}
	if len(sets) > 0 {
		return sets[0].ID
	}
	return ""
}

// ValidVariableName reports whether name can be referenced as ${name} or as the
// name portion of ${name:fallback}. Keeping the grammar identifier-like makes references
// unambiguous and portable across the GUI, YAML, and future clients.
func ValidVariableName(name string) bool {
	return variables.ValidName(name)
}
