# GUI smoke tests (Playwright)

Boots the **built** frontend (`vite build` + `vite preview` on :4173) in
Chromium with a fake Wails bridge and checks every major surface still
works. Nothing here talks to the real game server.

## Run

    make -C gui e2e-deps   # once: downloads Playwright's Chromium
    make -C gui e2e        # tsc over e2e/ + playwright test

`npx playwright test --ui` (from `gui/frontend`) opens the interactive
runner; `npx playwright show-report` opens the last HTML report.

`make -C gui check` does **not** run this suite; CI runs it as the `e2e` job
in `.github/workflows/gui.yaml`.

## How it works

- `fake-backend.ts` installs `window.go.gui.GuiApp` and `window.runtime`
  before the page loads. Readers answer from `fixtures.ts`; writers record
  their calls; `ConnectNew`/`ConnectStored` emit a `conn:connected` event so
  the real store performs the screen transition.
- Tests push wire events (`backend.text(...)`, `backend.events(...)`) and
  read recorded calls (`backend.args("Send")`).
- `test.ts` fails a test on any `pageerror` or on the console line
  `fake-backend: unhandled GuiApp.<name>` — add new Go bindings to the fake
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
