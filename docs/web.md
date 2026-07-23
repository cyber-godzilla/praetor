# Shared web mode

`praetor-web` is Praetor's headless browser shell. It serves the same Svelte
frontend used by the Wails desktop application and runs the same Go client,
protocol parser, Lua engine, map/compass renderers, config, secure credential storage,
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
--listen address    web listener (default 127.0.0.1:8787)
--debug             include debug protocol events
--tls-cert path     TLS certificate-file override (requires --tls-key)
--tls-key path      TLS private-key-file override (requires --tls-cert)
--insecure-http     disable default TLS and serve plaintext HTTP
--version           print the version and exit
```

HTTPS is the default. When neither TLS file is supplied, Praetor creates and
reuses a basic self-signed certificate under its state directory. Supplying
both TLS files overrides that automatic pair. Plaintext HTTP is available only
through the explicitly named `--insecure-http` option. A partial certificate
pair or a combination of certificate files and `--insecure-http` fails startup.
Open the default listener at `https://127.0.0.1:8787/`; a browser warning is
expected for the automatic self-signed certificate.

`PRAETOR_WEB_PASSWORD` must be present and non-empty. Praetor derives its
verifier before binding the listener and removes the variable from its own
environment. It does not write the password to config, state, logs, HTML, or
browser storage. Processes running as the same OS user may still be able to
observe a process's startup environment, so the service account is part of the
trust boundary. Rotate the password by restarting the process with a new value;
the restart also invalidates all browser sessions.

`PRAETOR_WEB_PASSWORD` is only the browser-access password. Do not reuse it as
the encrypted TEC credential-store key described below.

For a release-style Linux amd64 artifact with no GTK/WebKit dependency:

```sh
make web-linux-amd64
file praetor-web-linux-amd64
ldd praetor-web-linux-amd64   # expected to report that it is not dynamically linked
```

## Security boundary

The default listener is deliberately loopback-only and uses the automatically
generated self-signed TLS certificate. To listen on a trusted private LAN, bind
a private interface and restrict the port at the host/router firewall:

```sh
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 192.168.1.20:8787
```

That connection is encrypted, but a browser will warn because the automatic
certificate is self-signed. Accepting an unverified certificate warning does
not authenticate the server against an active attacker. Use a certificate
trusted by the client devices, or put a trusted HTTPS reverse proxy in front of
Praetor, when authenticated TLS is required. The preshared-password mode is not
suitable for direct Internet exposure.

Plaintext is an explicit compatibility/development override:

```sh
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 127.0.0.1:8787 --insecure-http
```

`--insecure-http` exposes the web password, TEC login credentials, commands,
and game text to anything able to inspect the network path. Avoid it on a LAN.

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

## Automatic self-signed HTTPS

On first startup without certificate overrides, Praetor creates:

```text
<state>/tls/praetor-web-self-signed.crt
<state>/tls/praetor-web-self-signed.key
```

The TLS directory and key are private to the service user. The certificate is
an ECDSA server certificate valid for five years and is renewed during a later
startup when fewer than 30 days remain. Praetor reuses the pair between
restarts, logs its SHA-256 fingerprint, and requires TLS 1.2 or newer.

When generated, the certificate automatically includes `localhost`, loopback
addresses, a concrete host or IP from `--listen`, and the machine hostname when
available. No SAN argument is required. These inferred names reduce avoidable
hostname errors, but do not make the self-signed certificate trusted. The
automatic certificate is intended to prevent default cleartext transport, not
to replace a certificate authority.

Praetor refuses to use a partial or malformed automatic pair, a symlink in
place of either file, or a private key accessible by group/other users. To
intentionally regenerate it, stop Praetor and remove both automatic files. A
new fingerprint will be produced on the next startup.

The state directory must be persistent in a service or container. An ephemeral
state directory produces a different certificate and browser warning after
each restart.

## Trusted certificate override

Use a certificate trusted by the phones and computers that will open the
client by supplying both override paths. For example, `mkcert` can issue a
private-LAN certificate after its CA has been installed on those devices:

```sh
mkcert -cert-file praetor.pem -key-file praetor-key.pem \
  praetor.home.arpa 192.168.1.20
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 192.168.1.20:8787 \
  --tls-cert praetor.pem --tls-key praetor-key.pem
```

Open `https://praetor.home.arpa:8787/`. HTTPS makes the session cookie Secure
and enables browser features such as native notifications and the Clipboard
API. Praetor validates that the supplied certificate and key form a usable
pair before starting the listener and does not create automatic TLS files in
this mode.

## HTTPS with Caddy

Praetor deliberately does not trust `X-Forwarded-Proto`; trusting arbitrary
forwarded headers would weaken same-Origin and Secure-cookie handling. The
default automatic certificate encrypts the loopback backend hop while Caddy
presents its LAN-trusted certificate:

```sh
PRAETOR_WEB_PASSWORD='choose-a-long-random-password' \
  ./praetor-web --listen 127.0.0.1:8787
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

`tls_insecure_skip_verify` is limited here to the loopback connection to the
automatic self-signed backend; never use that setting for a remote upstream.
An operator-provided backend certificate trusted by Caddy is stronger and can
be selected with `--tls-cert` and `--tls-key`. Point the LAN DNS name at the
host and install/trust Caddy's local root CA on each browser device. Caddy
handles the WebSocket upgrade automatically. Keep port 8787 inaccessible from
other hosts; expose only the proxy's HTTPS port.
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
- A settings change, script reload, mode switch, credential-store mutation, Kudos update,
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

## Files, credentials, logs, and backups

Run `praetor-web` as the ordinary OS user that owns the Praetor profile. Script
and transcript paths shown in a remote browser refer to the **server host**, not
the phone or desktop running that browser.

Praetor resolves its config, data, and state directories independently at
startup. An exact Praetor override wins over the corresponding XDG parent; if
neither is set, the normal home-directory fallback is used:

| Purpose | Exact application-directory override | XDG parent fallback | Home-directory fallback |
|---|---|---|---|
| Config | `PRAETOR_CONFIG_DIR` | `$XDG_CONFIG_HOME/praetor` | `~/.config/praetor` |
| Persistent Lua data | `PRAETOR_DATA_DIR` | `$XDG_DATA_HOME/praetor` | `~/.local/share/praetor` |
| Application state | `PRAETOR_STATE_DIR` | `$XDG_STATE_HOME/praetor` | `~/.local/state/praetor` |

The exact `PRAETOR_*_DIR` values name the application directory itself;
Praetor does not append `/praetor`. The XDG variables name parent directories,
so Praetor does append `/praetor` to them. Setting one exact override does not
change how the other two directories are resolved.

The resulting profile contains these files and directories:

| Data | Resolved location |
|---|---|
| Shared configuration | `<config>/config.yaml` |
| Default scripts | `<config>/scripts/` |
| Default session transcripts | `<config>/logs/` |
| Notes and exports | `<config>/notes/` and `<config>/exports/` |
| Lua persistent state | `<data>/persistent_state.json` |
| Application log | `<state>/tec.log` and `.1`, or retained `<state>/tec_YYYY-MM-DD_HH-MM-SS*.log` |
| Default encrypted credential file | `<state>/credentials/credentials.enc` |
| Automatic self-signed TLS pair | `<state>/tls/praetor-web-self-signed.{crt,key}` |

If `config.yaml` does not exist, Praetor creates the config directory, default
script directory, and a default configuration. A service user therefore needs
write access to the resolved profile directories.

### Environment variables

These are all of the environment-variable inputs used specifically by the web
startup and its shared GUI bootstrap:

| Variable | Requirement and effect |
|---|---|
| `PRAETOR_WEB_PASSWORD` | Required and non-empty. Authenticates browsers. Read and removed from Praetor's environment before the listener binds. |
| `PRAETOR_CONFIG_DIR` | Optional exact config application directory. Takes precedence over `XDG_CONFIG_HOME`. |
| `PRAETOR_DATA_DIR` | Optional exact persistent-data application directory. Takes precedence over `XDG_DATA_HOME`. |
| `PRAETOR_STATE_DIR` | Optional exact application-state directory. Takes precedence over `XDG_STATE_HOME`. |
| `XDG_CONFIG_HOME` | Optional config parent used when `PRAETOR_CONFIG_DIR` is unset; `/praetor` is appended. |
| `XDG_DATA_HOME` | Optional data parent used when `PRAETOR_DATA_DIR` is unset; `/praetor` is appended. |
| `XDG_STATE_HOME` | Optional state parent used when `PRAETOR_STATE_DIR` is unset; `/praetor` is appended. |
| `PRAETOR_CREDENTIALS_KEY` | Required only when `credentials.backend` is `encrypted_file` and `credentials.encrypted_file.key_env` retains its default. Must decode from base64 to exactly 32 bytes. |

`credentials.encrypted_file.key_env` may name a different environment variable;
that configured variable replaces `PRAETOR_CREDENTIALS_KEY` for key delivery.
Praetor reads it once and removes it from its environment after initializing
the credential store.

Directory overrides, credential backend selection, secret variables, listener
arguments, and TLS arguments are startup inputs and require a process restart
to change. Paths inside `config.yaml` for script directories, the encrypted
credential file, and session transcripts support `~/` and `$ENV_VAR`
expansion.

### Generic persistent service profile

The web binary does not have a separate configuration schema: it reads the
same complete `config.yaml` as the TUI and Wails applications. The
[configuration reference](configuration.md) documents every shared field. A
supervisor, container runtime, init system, or direct shell launch can use a
split persistent layout such as:

```text
/srv/praetor/
├── config/
│   ├── config.yaml
│   └── notes/
├── data/
├── state/
│   ├── credentials/
│   └── tls/                     # generated self-signed certificate and key
├── scripts/
└── session-logs/
```

Pass the non-secret directories and startup secrets through that environment's
normal configuration and secret facilities:

```text
PRAETOR_CONFIG_DIR=/srv/praetor/config
PRAETOR_DATA_DIR=/srv/praetor/data
PRAETOR_STATE_DIR=/srv/praetor/state
PRAETOR_WEB_PASSWORD=<independent-long-random-browser-password>
PRAETOR_CREDENTIALS_KEY=<base64-encoded-32-byte-key>
```

Do not commit the last two values or place them directly in a service command
line. `PRAETOR_CREDENTIALS_KEY` can be omitted when account persistence is
disabled or a usable OS keyring is selected. The matching deployment-specific
portion of `/srv/praetor/config/config.yaml` is:

```yaml
credentials:
  backend: encrypted_file
  encrypted_file:
    path: /srv/praetor/state/credentials/credentials.enc
    key_env: PRAETOR_CREDENTIALS_KEY

scripts:
  - /srv/praetor/scripts

logging:
  app:
    level: info
    max_size_mb: 5
    retain: false
  session:
    enabled: true
    path: /srv/praetor/session-logs
```

All omitted settings retain their shared defaults and can be managed through
the authenticated web Settings UI where exposed. Keep one writable profile per
process: multiple Praetor processes must not share these directories or try to
own the same TEC session.

Changing the logging directory while transcript logging is enabled closes the
old transcript and starts a new timestamped transcript immediately. Enabling or
disabling transcript logging also applies immediately in web, Wails, and TUI
shells. `retain` instead selects the application-log writer during
startup: enable it to preserve dated `tec_*.log` segments indefinitely, then
restart the process. Select `debug` to include exact received and sent
application lines; the default `info` level retains only lifecycle and
operational records.

Desktop installs default to the operating-system keyring. A headless service
normally cannot access or unlock a desktop keyring session, so it should select
the encrypted-file backend explicitly, as in the generic service configuration
above. An empty `credentials.encrypted_file.path` uses
`<state>/credentials/credentials.enc`.

Supply an independent base64-encoded 32-byte key through the service secret
manager:

```sh
openssl rand -base64 32
```

Praetor removes the configured credential-key variable from its own environment
after initializing the store. The encrypted account map uses a versioned
AES-256-GCM envelope, a fresh nonce for every write, authenticated decryption,
and atomic mode-0600 replacement. Missing, malformed, or incorrect keys fail
startup. The application never falls back to a plaintext credential file or
to a different backend.

When **Remember this account** is selected, Praetor authenticates and opens the
TEC WebSocket before attempting persistence. A storage failure therefore
leaves the shared game connected and produces an **Account not remembered**
warning. The login screen disables Remember when storage is known to be
unavailable, and distinguishes that state from a valid empty account list.
Saved usernames appear in the existing account selector; switching accounts
still requires disconnecting the one shared TEC session first.

Back up `config.yaml`, script directories, Lua persistent data, and any desired
session logs. Keyring-backed credentials remain managed by the OS keyring. For
the encrypted-file backend, back up `credentials.enc` and its secret-manager
key separately; neither is useful without the other. Losing or rotating the
key without re-encrypting the file makes saved accounts unrecoverable, but does
not affect scripts, configuration, logs, or interactive TEC login. Only one
Praetor process should use a given resolved profile at a time.

The automatic TLS pair can be regenerated because browsers are not expected to
trust it by default. Back up both files together only if retaining its stable
fingerprint or an existing browser exception matters; never back up or copy the
private key without protecting it as a secret.

## systemd user service

Install the binary for the user, then create a mode-0600 environment file:

```sh
install -Dm755 praetor-web-linux-amd64 "$HOME/.local/bin/praetor-web"
install -Dm600 /dev/null "$HOME/.config/praetor/web.env"
printf '%s\n' 'PRAETOR_WEB_PASSWORD=replace-with-a-long-random-password' \
  > "$HOME/.config/praetor/web.env"
# Required only when credentials.backend is encrypted_file:
printf 'PRAETOR_CREDENTIALS_KEY=' >> "$HOME/.config/praetor/web.env"
openssl rand -base64 32 >> "$HOME/.config/praetor/web.env"
chmod 0600 "$HOME/.config/praetor/web.env"
install -Dm644 packaging/systemd/praetor-web.service \
  "$HOME/.config/systemd/user/praetor-web.service"
systemctl --user daemon-reload
systemctl --user enable --now praetor-web.service
```

The supplied unit binds to loopback and uses automatic self-signed HTTPS. It can
be used as the encrypted backend for the Caddy setup above without an
`ExecStart` override. To listen directly on a private LAN, create an override
and choose a specific private address:

```sh
systemctl --user edit praetor-web.service
```

```ini
[Service]
ExecStart=
ExecStart=%h/.local/bin/praetor-web --listen 192.168.1.20:8787
```

That direct listener still uses automatic self-signed HTTPS. Plaintext requires
adding `--insecure-http` deliberately. To use an operator-managed certificate,
add both `--tls-cert` and `--tls-key` paths instead; the service user must be
able to read them.

After changing the password file, restart the service:

```sh
systemctl --user restart praetor-web.service
```

## Operational checks

`GET /healthz` is intentionally unauthenticated and returns only process
liveness. It does not reveal the version, account names, TEC state, or game
text.
