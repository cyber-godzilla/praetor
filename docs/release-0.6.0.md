# 0.6.0 Release Readiness

## Deferred to 0.7.0

The 0.7.0 feature list currently includes:

- Desktop Tray and unread-status indicator
- Speech-to-text command input
- Complete Theme Support
- Mobile-first `praetor-web` experience (cornerstone)

Mobile support is a cornerstone of 0.7.0, not a narrow responsive-CSS pass.
Build on the 0.6.0 mobile dock, touch scrolling, safe-area handling, software-
keyboard correction, and mobile preferences with phone- and tablet-first
navigation and view composition, per-device preferences, easier install and
connection onboarding, resilient resume/reconnect behavior after a device
sleeps or backgrounds the browser, accessible touch targets, and automated
coverage for iOS Safari and Android Chrome in portrait and landscape.

Keep 0.6.0 mobile work deliberately small and regression-focused. Safe
candidates are an explicit mobile send-key hint on the command input, complete
notch/safe-area viewport metadata after phone verification, and a phone-sized
browser smoke test covering web unlock, account/character selection, command
entry, tabs, Views, Actions, Modes, Menu, and modal scrolling. Broader mobile
navigation, per-device state, installability, and lifecycle work remain 0.7.0.

Desktop Tray and its unread-status indicator are no longer part of 0.6.0.
Preserve the agreed lifecycle and red/green/orange indicator contract for
0.7.0, but do not ship a partial tray implementation in this release.

Speech-to-text command input is also deferred to 0.7.0. Preserve the agreed
push-to-talk, review-before-send, privacy, and platform-recognizer boundaries;
do not add a partial microphone control to 0.6.0.

Before implementation resumes, choose and approve a supported native tray
host. The current Wails v2 shell has no supported system-tray API, so 0.7.0
must explicitly plan either a Wails v3 migration or a maintained native tray
adapter.

## Discord notifications (implemented)

The existing health, fatigue, matched-text, and opt-in Lua notification paths
can also deliver to one optional Discord webhook. Discord is disabled by
default and each category has an independent destination switch. The shared
GUI/web settings screen password-masks the webhook and provides a bounded
save-and-test operation. Delivery is asynchronous, suppresses Discord mentions,
accepts only official HTTPS webhook URLs, and never includes the webhook token
in logs or returned errors. Configurations containing a webhook are saved with
owner-only permissions.

## Script Manager and Lua sandbox (implemented)

The 0.6.0 Script Manager is narrowed to Local Directories plus anonymously
readable HTTPS Git repositories. `praetor-scripts` is the suggested source;
managed repositories install disabled and Restricted, track the highest stable
semantic-version Git tag when available (otherwise the remote default branch),
and use staged, validated, atomic updates with rollback.

Every managed source supports a manual one-click check-and-update transaction.
When an update is available and no Lua mode is active, Praetor temporarily
unloads the source, applies and reloads the update, and restores its prior
enabled state. Background startup checks remain asynchronous, bounded, and
notification-only: they default on for the suggested `praetor-scripts` source
and remain opt-in for arbitrary sources. Praetor never installs an update from
a background check.

ZIP sources, host-specific release APIs, prerelease tracking, plain-HTTP
confirmation flows, authenticated/private sources, and per-script dependency
management are deferred to 0.7.0. Enabling managed code depends on the 0.6.0
Restricted/Trusted Lua sandbox.

The shared GUI/web frontend includes configuration migration, suggested and
arbitrary HTTPS Git installs, stable-tag/default-branch selection, staged
validation, atomic updates with rollback, manual and startup checks, ordering,
confirmed removal, and the combined managed/local editor. Managed code can be
enabled explicitly. Restricted sources run in isolated Lua states with safe
standard libraries and source-scoped imports; Trusted sources retain the legacy
full standard library. Trust and enablement changes rebuild the runtime
transactionally and require confirmation before stopping active automation.

## Character-aware display profiles (dependency satisfied)

Character-scoped content lives in a UUID-addressed profile registry. The global
`Default` profile is the fallback and creation template. New characters copy
its variables once, after which each variable list is independent. Custom Views,
Action Sets, selected Action Set, Quick Cycle, high-priority
commands, display placement, and Lua persistent data switch together when a
character becomes active. Legacy configuration migrates into Default.

Players manage character profiles under **Esc → Character Profiles**. Each
non-default profile has an account and positive character number. After a
successful `play N` command—or `play` followed by `N`—Praetor selects the
matching profile for the logged-in account and broadcasts the switch to every
attached frontend. Detection follows successfully dispatched expanded commands,
including `;;` / `&&` PraetorScript steps and Lua `praetor_script()` calls. A
successful `sleep` command returns to Default. An automatic transition stops an
active Lua mode because the character-changing game command has already been
sent. Players can disable automatic handling of both commands in the profile
manager. Character numbers therefore only need to be unique within
an account. The automation-bar character chip and `/character [name|global]`
provide manual switching; manual switching is guarded while `/play` or Lua
automation is active. Active selection is deliberately session-only and starts
at Default after login or restart. Profile names are case-insensitively unique,
deletion is confirmed, and failed saves roll back.

Newly added profiles live-inherit Default Action Sets, Quick Cycle, and
high-priority commands; variables inherit per key. Players can explicitly
customize a collection to an empty list. Action Sets support reusable stable-ID
membership, profile-isolated edits, independent copy, reference-only removal,
and confirmed deletion of every copy with the same ID. Inherited custom
components receive fresh UUIDs.

Lua modes can read and synchronously write the active character's Variables.
Player-facing preferences and resumable workflow inputs belong there;
persistent Lua data is reserved for accumulated long-term records rather than
private mode steps. `state.unpersist` supports one-time migration of legacy
workflow keys without leaving stale entries in Persistent Data.

This supplies the authoritative account/character switch that character-aware
display profiles require. The global Default display profile already uses UUID
sidebars and ordered component-ID lists, supports any number of sidebars, and
saves drag/drop placement immediately. Its searchable Bench shows only
unplaced views; deleted-sidebar views return there in order and can
be dragged into a new destination. The editor keeps that Bench in a compact
independently scrolling right-hand column and resolves visible titles from
view metadata rather than exposing UUIDs. View cards also show their explicit
functional type. View creation/configuration now lives separately under
**View Library**, while **Layout** only controls placement and ordering. When
width is constrained, outer sidebars collapse to title-only rails without
rendering their views; a toast reports the change. Explicit Collapse and Expand
controls save manual state with each character layout, while responsive state
remains temporary.
Each non-default character inherits an independent display profile whose
filtered-text placements are remapped to that character's component UUIDs;
switching characters now changes content and placement together without
dropping the tab bar. Sidebar collapse controls retain the pre-0.6 floating
frame title and SVG triangle treatment.

## Wound-language catalogue (provisional; accepted for 0.6.0)

Wound highlighting ships in 0.6.0 with verified combat phrases and the observed
subset of character-inspection phrases. Unknown strings remain unchanged. A
complete catalogue across all three output contexts remains follow-up work and
is not a 0.6.0 release blocker:

1. Combat-result text when damage is applied
2. Casual character inspection (`look`)
3. Healer examination and treatment output

The combat-result catalogue is complete except for the unknown fifth-degree
burn phrase. Character-inspection logs currently verify these provisional
severity slots:

| Severity | Cutting | Bruising | Piercing | Burning |
|----------|---------|----------|----------|---------|
| 1 | `is scratched` | `is bruised` | `is punctured` | **TBD** |
| 2 | `is badly scratched` | `has a large bruise` | `is badly punctured` | **TBD** |
| 3 | `is slashed` | `has a large black bruise` | `is pierced` | **TBD** |
| 4 | `is slashed open` | `is badly bruised` | **TBD** | **TBD** |
| 5 | **TBD** | `is completely black and blue` | **TBD** | **TBD** |

These severity assignments are placeholders based on the observed wording and
ordering. Only `cut` → `is badly scratched` and `bruise` → `has a large bruise`
have clean combat-to-inspection correlations in the retained logs.

A retained healer log also verifies this combined diagnosis shape:

`His right shoulder has a large black bruise. It is not bleeding. There are some broken bones. His right shoulder is absolutely filthy. This part needs new bandages. There are no stitches on this part.`

That confirms real healer wording without implying a complete severity or
treatment catalogue.

### Follow-up catalogue work

- [ ] Capture or find a primary game-log/wiki reference for every **TBD** slot.
- [ ] Confirm the exact damage-severity mapping for all inspection phrases.
- [ ] Confirm fracture inspection output. An archived example suggests
      `is mangled` plus `Shards of bone penetrate the flesh of ...`, but it
      needs a repeatable or primary reference before becoming executable data.
- [ ] Catalogue healer examination strings for cuts, bruises/fractures,
      punctures, burns, bleeding, cleanliness, bandages, splints, and stitches.
- [ ] Add red-green fixtures for every confirmed phrase and important negative
      cases in each context.
- [ ] Replace provisional severity assignments with confirmed values.
- [ ] Prepare a wiki-ready summary for the maintainer to publish manually. Do
      not publish or edit the wiki from the release workflow.

Unknown placeholders are documentation only. Praetor must leave unknown text
unchanged rather than guess and produce false highlights.
