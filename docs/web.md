# Shared web mode

`praetor-web` is Praetor's headless browser shell. It serves the same Svelte
frontend used by the Wails desktop application and runs the same Go client,
protocol parser, Lua engine, map/compass renderers, config, keyring integration,
logging, and persistent state used by the other shells.

One process owns one TEC session. Every authenticated browser is an equal
operator of that session; web authentication is not a per-user permission
system.

## Build and start

The normal build embeds a production frontend in the Go binary:

```sh
make web
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 127.0.0.1:8787
```

Available startup options are:

```text
--listen address   HTTP listener (default 127.0.0.1:8787)
--debug            include debug protocol events
--tls-cert path    serve HTTPS with this certificate (requires --tls-key)
--tls-key path     serve HTTPS with this private key (requires --tls-cert)
--version          print the version and exit
```

`PRAETOR_WEB_PASSWORD` must be present and non-empty. Praetor derives its
verifier before binding the listener and removes the variable from its own
environment. It does not write the password to config, state, logs, HTML, or
browser storage. Processes running as the same OS user may still be able to
observe a process's startup environment, so the service account is part of the
trust boundary. Rotate the password by restarting the process with a new value;
the restart also invalidates all browser sessions.

For a release-style Linux amd64 artifact with no GTK/WebKit dependency:

```sh
make web-linux-amd64
file praetor-web-linux-amd64
ldd praetor-web-linux-amd64   # expected to report that it is not dynamically linked
```

## Security boundary

The default listener is deliberately loopback-only. To expose plaintext HTTP
on a trusted private LAN, bind a private interface and restrict the port at the
host/router firewall:

```sh
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 192.168.1.20:8787
```

Plain HTTP exposes the web password, TEC login credentials, commands, and game
text to anything able to inspect the network path. The preshared-password mode
is not suitable for direct Internet exposure. Prefer loopback plus HTTPS, even
on a wireless LAN.

The server provides the following application-level protections:

- a mandatory startup-only password with constant-time digest comparison;
- rate limits per source address and across the process;
- opaque, in-memory browser sessions with a 12-hour idle expiry;
- `HttpOnly`, `SameSite=Strict` cookies (`Secure` when served through TLS);
- same-Origin enforcement on login, mutations, and the event WebSocket;
- per-session CSRF tokens on every mutation;
- bounded request bodies, commands, replay history, browser sessions, and
  WebSocket queues;
- restrictive CSP, frame denial, no-referrer, and no-sniff headers; and
- disconnect-and-resnapshot behavior for a slow or out-of-sequence browser.

No forwarded host or scheme header is trusted. A reverse proxy should preserve
the original `Host` header and must not rewrite the browser-facing Origin.

## Direct HTTPS

Use a certificate trusted by the phones and computers that will open the
client. For example, `mkcert` can issue a private-LAN certificate after its CA
has been installed on those devices:

```sh
mkcert -cert-file praetor.pem -key-file praetor-key.pem \
  praetor.home.arpa 192.168.1.20
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 192.168.1.20:8787 \
  --tls-cert praetor.pem --tls-key praetor-key.pem
```

Open `https://praetor.home.arpa:8787/`. HTTPS makes the session cookie Secure
and enables browser features such as native notifications and the Clipboard
API.

## HTTPS with Caddy

Praetor deliberately does not trust `X-Forwarded-Proto`; trusting arbitrary
forwarded headers would weaken same-Origin and Secure-cookie handling. Run the
loopback hop with TLS as well, then let Caddy present its LAN-trusted
certificate. The backend certificate can be a separate local/self-signed
certificate because the hop never leaves the host:

```sh
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 127.0.0.1:8787 \
  --tls-cert backend.pem --tls-key backend-key.pem
```

An internal-CA Caddy configuration is:

```caddyfile
praetor.home.arpa {
    tls internal
    reverse_proxy https://127.0.0.1:8787 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

`tls_insecure_skip_verify` is limited here to the loopback backend hop; never
use that setting for a remote upstream. Point the LAN DNS name at the host and
install/trust Caddy's local root CA on each browser device. Caddy handles the
WebSocket upgrade automatically. Keep port 8787 inaccessible from other hosts;
expose only the proxy's HTTPS port.
Browser-native notifications and the Clipboard API generally require this
secure-context deployment (loopback is also treated as secure by browsers).

## Browser and game sessions

The login screen shown first is the Praetor web-password gate. It is separate
from the subsequent TEC account screen and never prefills or stores TEC
credentials.

- **Sign out of web UI** invalidates only that browser's opaque session.
- **Disconnect shared game** closes the one TEC session and returns every
  browser to TEC account selection/login.
- Closing a tab, changing networks, or reconnecting a browser does not reconnect
  or disconnect TEC. The browser receives an atomic bounded-history snapshot,
  then resumes ordered live updates.
- A settings change, script reload, mode switch, keyring mutation, Kudos update,
  persistent-data clear, or command affects the shared process.
- Input drafts, command history, active tab, scroll position, unread markers,
  text selection, collapsed panels, and browser-notification permission remain
  local to each browser.

All authenticated browsers are equally authorized. Do not share the web
password with someone who should not be able to send commands, alter settings,
manage stored TEC accounts, or clear persistent state.

## Mobile browser preferences

The web Settings modal includes five preferences for the responsive mobile layout:

- **Show Actions / Modes / Menu row on mobile** controls the three-button row
  beneath the map and compass. It is enabled by default.
- **Show tab selector on mobile** controls the All/Metrics/custom-tab row. It
  is enabled by default. When disabled, the Menu button moves to the far right
  of the compact HP/status row so settings remain reachable. The normal
  connected state uses only its green indicator on mobile; disconnected/error
  text remains visible so an abnormal connection state is not hidden.
- **Hide map and compass while command input is active** removes the navigation
  region while the mobile command field has focus, leaving more room for game
  text and the software keyboard. The region returns when input focus leaves.
  While the keyboard opens, the client also waits for the mobile visual viewport
  to settle and returns the outer browser page to the top if the browser panned
  it. This does not change the session output pane's scrollback position.
- **Lowercase the first command letter on mobile** requests that the software
  keyboard not capitalize commands and also normalizes the first cased letter
  as a browser-side fallback. Leading spaces, punctuation, numbers, and slash
  prefixes are preserved; the remainder of the command is not changed.
- **Mobile output text size** controls game-output text independently from the
  desktop output size. It accepts values down to 6 CSS pixels for denser phone
  scrollback. Existing configurations initially inherit their desktop value.

These are shared server settings: saving them in one authenticated browser
persists them in `config.yaml` and broadcasts the updated configuration to the
other connected browsers. The toolbar and navigation options apply to the web
frontend's mobile-width layout, including the independent output size.
First-letter normalization also applies to a
coarse-pointer web device whose viewport is wider than the mobile breakpoint.
These mobile-layout preferences do not affect the TUI or native Wails layout.

## Files, keyring, logs, and backups

Run `praetor-web` as the ordinary OS user that owns the Praetor profile. Script
and transcript paths shown in a remote browser refer to the **server host**, not
the phone or desktop running that browser.

The process uses the normal XDG locations:

```text
$XDG_CONFIG_HOME/praetor/config.yaml       (default ~/.config/praetor)
$XDG_CONFIG_HOME/praetor/scripts/
$XDG_CONFIG_HOME/praetor/logs/             (default session transcripts)
$XDG_DATA_HOME/praetor/                    (Lua persistent data)
$XDG_STATE_HOME/praetor/tec.log            (application log)
```

Service deployments that already have exact application directories can set
`PRAETOR_CONFIG_DIR`, `PRAETOR_DATA_DIR`, and `PRAETOR_STATE_DIR`. Each value
names the application directory itself; Praetor does not append `/praetor` to
these overrides. When an override is absent, the corresponding normal XDG path
above remains in effect.

Changing the logging directory while transcript logging is enabled closes the
old transcript and starts a new timestamped transcript immediately. Enabling or
disabling logging also applies immediately in web, Wails, and TUI shells.

A headless service may not have access to a desktop keyring session. In that
case, stored-account operations return an error, while an unstored TEC login
remains available. Praetor never falls back to a plaintext credential file.

Back up `config.yaml`, script directories, Lua persistent data, and any desired
session logs. Credential backups are managed by the OS keyring and are not part
of Praetor's filesystem backup. Only one Praetor process should use a given XDG
profile at a time.

## systemd user service

Install the binary for the user, then create a mode-0600 environment file:

```sh
install -Dm755 praetor-web-linux-amd64 "$HOME/.local/bin/praetor-web"
install -Dm600 /dev/null "$HOME/.config/praetor/web.env"
printf '%s\n' 'PRAETOR_WEB_PASSWORD=replace-with-a-long-random-password' \
  > "$HOME/.config/praetor/web.env"
chmod 0600 "$HOME/.config/praetor/web.env"
install -Dm644 packaging/systemd/praetor-web.service \
  "$HOME/.config/systemd/user/praetor-web.service"
systemctl --user daemon-reload
systemctl --user enable --now praetor-web.service
```

The supplied unit binds to loopback for local-only HTTP. For the Caddy setup
above, override `ExecStart` to add the two backend TLS flags. To use direct,
plaintext LAN mode, create an override and choose a specific private address:

```sh
systemctl --user edit praetor-web.service
```

```ini
[Service]
ExecStart=
ExecStart=%h/.local/bin/praetor-web --listen 192.168.1.20:8787
```

After changing the password file, restart the service:

```sh
systemctl --user restart praetor-web.service
```

## Operational checks

`GET /healthz` is intentionally unauthenticated and returns only process
liveness. It does not reveal the version, account names, TEC state, or game
text.

Before relying on a LAN deployment, verify:

1. a wrong password, missing cookie, wrong Origin, and missing CSRF token cannot
   read or mutate state;
2. a desktop and phone can join at different times and converge on the same
   output, gauges, map, compass, mode, and settings;
3. alternating commands are observed in server order;
4. signing one browser out leaves the other browser and TEC connected;
5. a shared-game disconnect is clearly confirmed and reaches every browser;
6. sleep/wake or network changes produce a browser resnapshot, not a TEC
   reconnect; and
7. application/session logs contain neither web nor TEC passwords.
