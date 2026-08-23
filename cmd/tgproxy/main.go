package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tgproxy/internal/rotatelog"
)

const version = "1.0.0"

type config struct {
	APIListen, APIKey, Upstream, MTGPath, MTGListen, MTGSecret, PublicHost, LogDir string
	TLSCert, TLSKey                                                                string
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func loadConfig() config {
	return config{
		APIListen: env("TGPROXY_API_LISTEN", ":8080"), APIKey: os.Getenv("TGPROXY_API_KEY"),
		Upstream: env("TGPROXY_UPSTREAM", "https://api.telegram.org"), MTGPath: env("TGPROXY_MTG_PATH", "/usr/local/bin/mtg"),
		MTGListen: env("TGPROXY_MTG_LISTEN", "0.0.0.0:443"), MTGSecret: os.Getenv("TGPROXY_MTG_SECRET"),
		PublicHost: os.Getenv("TGPROXY_PUBLIC_HOST"), LogDir: env("TGPROXY_LOG_DIR", "./logs"),
		TLSCert: os.Getenv("TGPROXY_TLS_CERT"), TLSKey: os.Getenv("TGPROXY_TLS_KEY"),
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(false)
	case "serve-all":
		err = serve(true)
	case "menu":
		err = menu()
	case "check":
		err = check(loadConfig())
	case "secret":
		fmt.Println(generateSecret())
	case "version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func usage() { fmt.Println("tgproxy {serve|serve-all|menu|check|secret|version}") }

func check(c config) error {
	if c.APIKey == "" {
		return errors.New("TGPROXY_API_KEY обязателен")
	}
	if len(c.APIKey) < 24 {
		return errors.New("TGPROXY_API_KEY должен быть не короче 24 символов")
	}
	if c.MTGSecret == "" {
		return errors.New("TGPROXY_MTG_SECRET обязателен")
	}
	if _, err := url.ParseRequestURI(c.Upstream); err != nil {
		return fmt.Errorf("TGPROXY_UPSTREAM: %w", err)
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("TLS cert и key задаются вместе")
	}
	return nil
}

func serve(withMTG bool) error {
	c := loadConfig()
	if err := check(c); err != nil {
		return err
	}
	if err := os.MkdirAll(c.LogDir, 0750); err != nil {
		return err
	}
	lw, err := rotatelog.New(c.LogDir, "tgproxy", 7)
	if err != nil {
		return err
	}
	defer lw.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, lw), nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var mtg *exec.Cmd
	if withMTG {
		mtg = exec.CommandContext(ctx, c.MTGPath, "simple-run", c.MTGListen, c.MTGSecret)
		mtg.Stdout, mtg.Stderr = lw, lw
		if err := mtg.Start(); err != nil {
			return fmt.Errorf("запуск MTProto: %w", err)
		}
		logger.Info("MTProto запущен", "listen", c.MTGListen, "pid", mtg.Process.Pid)
		go func() {
			if err := mtg.Wait(); err != nil && ctx.Err() == nil {
				logger.Error("MTProto остановился", "error", err)
			}
			stop()
		}()
	}

	handler, err := apiHandler(c, logger, nil)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: c.APIListen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	logger.Info("Bot API proxy запущен", "listen", c.APIListen, "tls", c.TLSCert != "")
	if c.TLSCert != "" {
		err = srv.ListenAndServeTLS(c.TLSCert, c.TLSKey)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func apiHandler(c config, logger *slog.Logger, transport http.RoundTripper) (http.Handler, error) {
	target, err := url.Parse(c.Upstream)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	if transport == nil {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ForceAttemptHTTP2: true, MaxIdleConns: 200, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ExpectContinueTimeout: time.Second}
	}
	proxy.Transport = transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("upstream error", "error", err)
		http.Error(w, "Telegram API unavailable", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"status":"ok"}`)
			return
		}
		provided := r.Header.Get("X-TGProxy-Key")
		if provided == "" {
			provided = r.URL.Query().Get("proxy_key")
			q := r.URL.Query()
			q.Del("proxy_key")
			r.URL.RawQuery = q.Encode()
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(c.APIKey)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !validTelegramPath(r.URL.Path) {
			http.Error(w, "invalid Telegram Bot API path", http.StatusBadRequest)
			return
		}
		logger.Info("api request", "method", r.Method, "path", redactedPath(r.URL.Path), "remote", remoteIP(r.RemoteAddr), "duration_ms", time.Since(start).Milliseconds())
		proxy.ServeHTTP(w, r)
	}), nil
}

func validTelegramPath(path string) bool {
	p := strings.TrimPrefix(path, "/")
	return strings.HasPrefix(p, "bot") || strings.HasPrefix(p, "file/bot")
}

func redactedPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, "bot") {
			parts[i] = "bot<redacted>"
		}
	}
	return strings.Join(parts, "/")
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	return addr
}

func generateSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "ee" + hex.EncodeToString(b) + hex.EncodeToString([]byte("google.com"))
}

func menu() error {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nTGProxy — управление\n1. Настроить (.env)\n2. Запустить\n3. Остановить\n4. Статус\n5. Ссылки MTProto\n6. Логи\n7. Проверить конфигурацию\n0. Выход\n> ")
		line, _ := in.ReadString('\n')
		switch strings.TrimSpace(line) {
		case "1":
			if err := wizard(in); err != nil {
				fmt.Println("Ошибка:", err)
			}
		case "2":
			runCompose("up", "-d", "--build")
		case "3":
			runCompose("down")
		case "4":
			runCompose("ps")
		case "5":
			printLinksFromEnv()
		case "6":
			runCompose("logs", "--tail=100", "-f")
		case "7":
			loadDotEnv(".env")
			if err := check(loadConfig()); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Конфигурация корректна")
			}
		case "0":
			return nil
		default:
			fmt.Println("Выберите пункт 0–7")
		}
	}
}

func wizard(in *bufio.Reader) error {
	ask := func(label, def string) string {
		fmt.Printf("%s [%s]: ", label, def)
		s, _ := in.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		return s
	}
	host := ask("Публичный IP или домен", env("TGPROXY_PUBLIC_HOST", "example.com"))
	mtPort := ask("Порт MTProto", "443")
	apiPort := ask("Порт Bot API", "8443")
	apiKey := randomHex(32)
	secret := generateSecret()
	content := fmt.Sprintf("TGPROXY_PUBLIC_HOST=%s\nTGPROXY_MTG_PORT=%s\nTGPROXY_API_PORT=%s\nTGPROXY_API_KEY=%s\nTGPROXY_MTG_SECRET=%s\n", host, mtPort, apiPort, apiKey, secret)
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		return err
	}
	fmt.Println("Создан .env. Ключ Bot API:", apiKey)
	return nil
}

func randomHex(n int) string { b := make([]byte, n); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func runCompose(args ...string) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Println("Ошибка docker compose:", err)
	}
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok && !strings.HasPrefix(strings.TrimSpace(k), "#") {
			_ = os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
}

func printLinksFromEnv() {
	loadDotEnv(".env")
	c := loadConfig()
	port := env("TGPROXY_MTG_PORT", "443")
	if c.PublicHost == "" || c.MTGSecret == "" {
		fmt.Println("Сначала выполните настройку")
		return
	}
	q := url.Values{"server": {c.PublicHost}, "port": {port}, "secret": {c.MTGSecret}}
	fmt.Println("Telegram:", "tg://proxy?"+q.Encode())
	fmt.Println("Web:", "https://t.me/proxy?"+q.Encode())
}
