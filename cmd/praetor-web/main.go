// Command praetor-web serves the shared Praetor client to authenticated LAN
// browsers. It is a headless shell over the same Wails-free application facade
// and game core used by the native GUI.
package main

import (
	"context"
	"crypto/tls"
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
	"github.com/cyber-godzilla/praetor/internal/webtls"
)

var version = ""

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:8787", "Web listen address")
	debug := flag.Bool("debug", false, "Enable debug events and logging")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file override (requires --tls-key)")
	tlsKey := flag.String("tls-key", "", "TLS private-key file override (requires --tls-cert)")
	insecureHTTP := flag.Bool("insecure-http", false, "Disable default TLS and serve plaintext HTTP")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if version == "" {
		version = versioninfo.Version
	}
	if *showVersion {
		fmt.Printf("praetor-web %s\n", version)
		return
	}
	tlsMode, err := resolveTLSMode(*tlsCert, *tlsKey, *insecureHTTP)
	if err != nil {
		log.Fatal(err)
	}

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

	certificateFile := *tlsCert
	privateKeyFile := *tlsKey
	useTLS := tlsMode != tlsModeInsecureHTTP
	if tlsMode == tlsModeAutomatic {
		material, err := webtls.Ensure(deps.StateDir, *listenAddr)
		if err != nil {
			log.Fatalf("automatic TLS: %v", err)
		}
		certificateFile = material.CertificateFile
		privateKeyFile = material.PrivateKeyFile
		if material.Generated {
			log.Printf("generated automatic self-signed TLS certificate %s", material.CertificateFile)
		}
		log.Printf("WARNING: using an automatically generated self-signed TLS certificate; traffic is encrypted, but browsers will not trust the server unless the certificate is accepted explicitly")
		log.Printf("automatic TLS SHA-256 fingerprint: %s", material.Fingerprint)
	} else if tlsMode == tlsModeExplicit {
		if _, err := tls.LoadX509KeyPair(certificateFile, privateKeyFile); err != nil {
			log.Fatalf("loading TLS certificate override: %v", err)
		}
	}

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
	if !useTLS {
		log.Printf("WARNING: --insecure-http is serving Praetor over plaintext HTTP on %s; passwords, commands, and game text are visible to the network path", listener.Addr())
		if !isLoopbackListener(listener.Addr()) {
			log.Printf("WARNING: plaintext HTTP is bound to a non-loopback address")
		}
	}

	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if useTLS {
		httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	errCh := make(chan error, 1)
	go func() {
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		log.Printf("Praetor web listening on %s://%s", scheme, listener.Addr())
		if useTLS {
			errCh <- httpServer.ServeTLS(listener, certificateFile, privateKeyFile)
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

type tlsMode uint8

const (
	tlsModeAutomatic tlsMode = iota
	tlsModeExplicit
	tlsModeInsecureHTTP
)

func resolveTLSMode(certificateFile, privateKeyFile string, insecureHTTP bool) (tlsMode, error) {
	if (certificateFile == "") != (privateKeyFile == "") {
		return 0, errors.New("--tls-cert and --tls-key must be provided together")
	}
	if insecureHTTP && certificateFile != "" {
		return 0, errors.New("--insecure-http cannot be combined with --tls-cert or --tls-key")
	}
	if insecureHTTP {
		return tlsModeInsecureHTTP, nil
	}
	if certificateFile != "" {
		return tlsModeExplicit, nil
	}
	return tlsModeAutomatic, nil
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
