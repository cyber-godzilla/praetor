# GUI smoke tests (Playwright)

Boots the **built** frontend (`vite build` + `vite preview` on :4321) in
Chromium with a fake Wails bridge and checks every major surface still
works. Nothing here talks to the real game server.

## Run

    make -C gui e2e-deps   # once: downloads Playwright's Chromium
    make -C gui e2e        # tsc over e2e/ + playwright test

`npx playwright test --ui` (from `gui/frontend`) opens the interactive
runner; `npx playwright show-report` opens the last HTML report.

`make -C gui screenshots` renders a populated client entirely from the fake
bridge and writes `gui/frontend/artifacts/ui-screenshots/praetor-main.png`,
`praetor-layout-callouts.png`, and `praetor-automation.png`. The layout image
contains numbered callouts only; its editable text legend lives in the wiki's
Praetor guide. The documentation scenario asserts that neither `ConnectNew` nor
`ConnectStored` was called, so it cannot open a game-server connection. CI
renders the same files and uploads them as the `praetor-ui-screenshots`
artifact.

Before Playwright starts, `gui/testdata/render-doc-graphics` parses sanitized
SKOOT 6/7/10 frames and runs Praetor's production minimap and compass renderers.
The screenshot therefore exercises the real map protocol/rendering path rather
than maintaining a separate hand-drawn approximation.

The rank-bonus screenshot is also production-backed:
`gui/testdata/render-doc-calculator` calls `internal/calc` to generate its
rank-bonus and training-cost fixture. The screenshot test asserts the canonical
1150/500 to 1150/1150 values before it writes the image, so illustrative fake
bridge values cannot leak into the documentation.

`make -C gui check` does **not** run this suite; CI runs it as the `e2e` job
in `.github/workflows/gui.yaml`, which fails the build on a flaky test (not
just an outright failure) via `--fail-on-flaky-tests`.

## How it works

- `fake-backend.ts` installs `window.go.gui.GuiApp` and `window.runtime`
  before the page loads. Readers answer from `fixtures.ts`; writers record
  their calls; `ConnectNew`/`ConnectStored` emit a `praetor:events` batch
  carrying `{ kind: "conn", conn: { state: "connected" } }` so the real store
  performs the screen transition.
- Tests push wire events (`backend.text(...)`, `backend.events(...)`) and
  read recorded calls (`backend.args("Send")`).
- Documentation screenshots use `backend.enterGame()`, which emits only the
  synthetic connected-state event needed to show the game screen. It never
  invokes either connector.
- `test.ts` fails a test on any `pageerror`, on the console line
  `fake-backend: unhandled GuiApp.<name>`, or on an unhandled promise
  rejection in the page (logged by the fake as
  `fake-backend: unhandledrejection ...`) — add new Go bindings to the fake
  deliberately (readers / writers / connectors in `fake-backend.ts`).
- Wire shapes are typed against `src/lib/types.ts`; a payload rename on the
  Go side fails `tsc` before any test runs.

## Gotchas

- **Splash:** auto-dismisses after 5 s; `backend.boot()` presses a key.
- **CRT effects** are off in the fixture — the roll overlay animates forever.
- **Tab:** `GameView` matches Tab on `e.code`. Playwright's `keyboard.press`
  sets it; the in-app browser harness used during development does not.
- **NumLock** cannot be set via Playwright, and `press("Numpad8")` reports
  `key: "8"` (NumLock on). The numpad test dispatches a synthetic keydown with
  `code: "Numpad8", key: "ArrowUp"` — what WebKitGTK delivers with NumLock off.
- **Update toast:** `CheckForUpdate` returns `available: false` so the
  2.5 s startup check never adds a toast.
- Test ids are added only where no accessible handle exists, named
  `e2e-<thing>`: `e2e-output`, `e2e-hint`, `e2e-toasts`.
- **Disconnected event drops:** the store drops text/status/graphics events
  while disconnected, so calling `backend.text(...)` before `backend.connect()`
  renders nothing — the test then fails on a missing-text assertion with no
  other clue. Connect first.
