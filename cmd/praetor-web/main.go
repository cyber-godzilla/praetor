// Command praetor-web serves the shared Praetor client to authenticated LAN
// browsers. It is a headless shell over the same Wails-free application facade
// and game core used by the native GUI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	appgui "github.com/cyber-godzilla/praetor/internal/gui"
	versioninfo "github.com/cyber-godzilla/praetor/internal/version"
	webapp "github.com/cyber-godzilla/praetor/internal/web"
	"github.com/cyber-godzilla/praetor/internal/webassets"
)

var version = ""

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	debug := flag.Bool("debug", false, "Enable debug events and logging")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file (requires --tls-key)")
	tlsKey := flag.String("tls-key", "", "TLS private-key file (requires --tls-cert)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if version == "" {
		version = versioninfo.Version
	}
	if *showVersion {
		fmt.Printf("praetor-web %s\n", version)
		return
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		log.Fatal("--tls-cert and --tls-key must be provided together")
	}
	useTLS := *tlsCert != ""

	auth, err := authFromEnvironment()
	if err != nil {
		log.Fatalf("web authentication: %v", err)
	}

	assets, err := webassets.FS()
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		log.Fatal("web assets are not built; run `make web-assets` or `make web`")
	}

	deps, err := appgui.Bootstrap(version, *debug)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer deps.Close()

	hub := webapp.NewHub(deps.Config.UI.Scrollback)
	deps.DesktopNotify.SetSink(client.NotificationSinkFunc(func(title, message string) {
		hub.Emit(appgui.EventChannel, []appgui.WireEvent{{
			Kind: appgui.KindNotify,
			Notify: &appgui.NotifyPayload{
				Title: title, Message: message,
			},
		}})
	}))
	app := appgui.NewGuiApp(deps, hub)
	server := webapp.NewServer(app, auth, hub, assets, log.Default())
	app.Start()

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *listenAddr, err)
	}
	if !isLoopbackListener(listener.Addr()) && !useTLS {
		log.Printf("WARNING: serving Praetor over plaintext HTTP on %s; passwords, commands, and game text are visible to the local network path", listener.Addr())
	}

	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		log.Printf("Praetor web listening on %s://%s", scheme, listener.Addr())
		if useTLS {
			errCh <- httpServer.ServeTLS(listener, *tlsCert, *tlsKey)
			return
		}
		errCh <- httpServer.Serve(listener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		log.Printf("received %s; shutting down", sig)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("web server stopped: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hub.Close()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("web shutdown: %v", err)
	}
	app.Disconnect()
}

func authFromEnvironment() (*webapp.AuthManager, error) {
	password := os.Getenv("PRAETOR_WEB_PASSWORD")
	if password == "" {
		return nil, errors.New("PRAETOR_WEB_PASSWORD is required and must not be empty")
	}
	auth, err := webapp.NewAuthManager(password)
	password = ""
	_ = os.Unsetenv("PRAETOR_WEB_PASSWORD")
	return auth, err
}

func isLoopbackListener(addr net.Addr) bool {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
