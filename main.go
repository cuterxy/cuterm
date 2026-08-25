// cuterm is a shared terminal server: run one binary, open the page in a
// browser, and create or attach to shell terminals that stay in sync across
// any number of clients.
//
// By default cuterm detaches itself into a background process and shows a
// system tray icon; use the tray menu to open the management page or quit.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/cuterxy/cuterm/internal/server"
	"github.com/cuterxy/cuterm/internal/terminal"

	"github.com/getlantern/systray"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

//go:embed web
var webFS embed.FS

func main() {
	cfg := loadConfig()
	defaultAddr := ":7681"
	if cfg.Port >= 1 && cfg.Port <= 65535 {
		defaultAddr = ":" + strconv.Itoa(cfg.Port)
	}
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	showVersion := flag.Bool("version", false, "print version and exit")
	foreground := flag.Bool("foreground", false, "run in the foreground instead of daemonizing")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	if !*foreground {
		daemonize()
	}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("load embedded web assets: %v", err)
	}

	mgr := terminal.NewManager()
	if cfg.Shell != "" {
		if err := mgr.SetShell(cfg.Shell); err != nil {
			log.Printf("config shell %q: %v", cfg.Shell, err)
		}
	}
	api := server.New(static, mgr)
	api.SetVersion(version)
	svc := &httpService{handler: api.Handler()}
	api.OnPortChange = func(port int) error {
		if err := svc.ChangePort(port); err != nil {
			return err
		}
		cfg.Port = port
		cfg.save()
		return nil
	}
	api.OnShellChange = func(shell string) error {
		if err := mgr.SetShell(shell); err != nil {
			return err
		}
		cfg.Shell = shell
		cfg.save()
		return nil
	}
	api.SetAppearance(server.Appearance{
		FontFamily: cfg.FontFamily,
		FontSize:   cfg.FontSize,
		Theme:      cfg.Theme,
		Scrollback: cfg.Scrollback,
	})
	api.OnAppearanceChange = func(a server.Appearance) error {
		cfg.FontFamily = a.FontFamily
		cfg.FontSize = a.FontSize
		cfg.Theme = a.Theme
		cfg.Scrollback = a.Scrollback
		cfg.save()
		return nil
	}
	api.SetLanguage(cfg.Language)
	api.OnLanguageChange = func(lang string) error {
		cfg.Language = lang
		cfg.save()
		traySetLanguage(lang)
		return nil
	}

	// Reverse connection to a cuterm-hub: the client is (re)started whenever
	// the hub address changes via the config page, and at startup if set.
	var hubMu sync.Mutex
	var hubC *hubClient
	applyHub := func(addr string) error {
		hubMu.Lock()
		defer hubMu.Unlock()
		if hubC != nil {
			hubC.Stop()
			hubC = nil
		}
		cfg.HubAddr = addr
		if addr != "" {
			if cfg.HubID == "" {
				cfg.HubID = newHubID()
			}
			name, err := os.Hostname()
			if err != nil || name == "" {
				name = cfg.HubID
			}
			hubC = newHubClient(addr, cfg.HubID, name, version, svc.localAddr)
			hubC.Start()
		}
		cfg.save()
		return nil
	}
	api.OnHubChange = applyHub
	api.HubStatus = func() (string, bool) {
		hubMu.Lock()
		defer hubMu.Unlock()
		if hubC == nil {
			return "", false
		}
		return cfg.HubAddr, hubC.Connected()
	}

	cleanup := func() {
		mgr.CloseAll()
		svc.Close()
	}

	// Clean up terminals on Ctrl-C / SIGTERM.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		systray.Quit()
	}()

	if err := svc.Listen(*addr); err != nil {
		cleanup()
		log.Fatalf("server: %v", err)
	}

	if cfg.HubAddr != "" {
		if err := applyHub(cfg.HubAddr); err != nil {
			log.Printf("hub %s: %v", cfg.HubAddr, err)
		}
	}

	fmt.Printf("cuterm %s\n", version)
	fmt.Printf("open %s in your browser\n", svc.URL())
	runTray(svc.URL, func() string { return svc.URL() + "/config.html" }, cfg.Language, cleanup)
}

// httpService owns the active HTTP server and can swap it onto a new port
// at runtime without dropping terminals.
type httpService struct {
	handler http.Handler

	mu   sync.Mutex
	srv  *http.Server
	addr string
}

// Listen starts serving on addr, replacing any previous listener. The old
// server is only closed after the new listener is up, so a failed bind
// leaves the service untouched.
func (svc *httpService) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	svc.mu.Lock()
	old := svc.srv
	svc.srv = &http.Server{Handler: svc.handler}
	svc.addr = addr
	srv := svc.srv
	svc.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != http.ErrServerClosed {
			log.Printf("http server on %s: %v", addr, err)
		}
	}()
	if old != nil {
		// Graceful close in the background: the request that triggered a
		// port change is still being served by the old server and must
		// finish first (a synchronous close would deadlock or cut it off).
		go func() { _ = old.Shutdown(context.Background()) }()
	}
	return nil
}

// ChangePort re-listens on the same host with a new port.
func (svc *httpService) ChangePort(port int) error {
	svc.mu.Lock()
	addr := svc.addr
	svc.mu.Unlock()
	host, cur, err := net.SplitHostPort(addr)
	if err != nil {
		host = ""
	}
	if cur == strconv.Itoa(port) {
		return nil // already on the requested port
	}
	return svc.Listen(net.JoinHostPort(host, strconv.Itoa(port)))
}

// URL returns the browser address of the management page.
func (svc *httpService) URL() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return "http://localhost" + displayAddr(svc.addr)
}

// localAddr returns the loopback dial address of the active HTTP server
// ("127.0.0.1:port"), used by the hub client to bridge tunnel connections
// into the local API. It follows runtime port changes.
func (svc *httpService) localAddr() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	_, port, err := net.SplitHostPort(svc.addr)
	if err != nil {
		return svc.addr
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// Close shuts down the active HTTP server.
func (svc *httpService) Close() {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.srv != nil {
		_ = svc.srv.Close()
	}
}

func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return addr
	}
	return ":" + addr
}

// newHubID returns the random node token sent to the hub in the hello; it
// is generated once and persisted in the config.
func newHubID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// fatal prints a message to stderr and exits with a non-zero status.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cuterm: "+format+"\n", args...)
	os.Exit(1)
}
