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
	"strconv"
	"strings"
	"syscall"
	"time"

	"tgproxy/internal/rotatelog"
)

const version = "1.1.1"

const placeholderHTML = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="robots" content="noindex,nofollow">
  <meta name="theme-color" content="#f6f0e8">
  <title>Анна &amp; Михаил — приглашение на свадьбу</title>
  <style>
    :root{--ink:#3e463b;--sage:#87927b;--cream:#f6f0e8;--paper:#fffdf9;--rose:#c99f95;--line:#dcd3c7}*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:var(--cream);color:var(--ink);font:16px/1.7 Georgia,"Times New Roman",serif}body:before{content:"";position:fixed;inset:0;pointer-events:none;opacity:.35;background:radial-gradient(circle at 10% 15%,rgba(201,159,149,.22),transparent 24%),radial-gradient(circle at 90% 80%,rgba(135,146,123,.18),transparent 28%)}.wrap{width:min(920px,calc(100% - 28px));margin:auto}.hero{min-height:100svh;display:grid;place-items:center;text-align:center;padding:60px 0}.card{position:relative;width:100%;overflow:hidden;padding:clamp(48px,9vw,100px) 24px;background:var(--paper);border:1px solid rgba(135,146,123,.25);box-shadow:0 24px 80px rgba(62,70,59,.12)}.card:before,.card:after{content:"";position:absolute;width:230px;height:230px;border:1px solid rgba(135,146,123,.4);border-radius:50%;transform:rotate(35deg)}.card:before{top:-155px;left:-80px}.card:after{right:-90px;bottom:-165px}.eyebrow{margin:0 0 34px;color:var(--sage);font:600 12px/1.3 Arial,sans-serif;letter-spacing:.32em;text-transform:uppercase}.names{margin:0;font-size:clamp(54px,12vw,112px);font-weight:400;line-height:.85;letter-spacing:-.06em}.amp{display:block;margin:16px 0;color:var(--rose);font-size:.48em;font-style:italic}.date{display:flex;align-items:center;justify-content:center;gap:18px;margin:44px auto 0;font-size:clamp(18px,3vw,26px);letter-spacing:.12em}.date:before,.date:after{content:"";width:64px;height:1px;background:var(--line)}.intro{max-width:620px;margin:40px auto 0;font-size:19px}.button{display:inline-block;margin-top:34px;padding:13px 30px;border:1px solid var(--sage);border-radius:999px;color:var(--ink);font:600 12px Arial,sans-serif;letter-spacing:.16em;text-decoration:none;text-transform:uppercase;transition:.2s}.button:hover{background:var(--sage);color:white}.section{padding:90px 0;text-align:center}.section h2{margin:0 0 16px;font-size:clamp(36px,7vw,62px);font-weight:400}.subtitle{max-width:590px;margin:0 auto 50px;color:#687064}.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}.item{padding:34px 20px;background:rgba(255,253,249,.72);border:1px solid var(--line)}.time{display:block;margin-bottom:8px;color:var(--rose);font-size:24px}.item h3{margin:0 0 5px;font-size:21px;font-weight:400}.item p{margin:0;color:#72776e}.place{padding:50px 24px;background:var(--sage);color:white}.place h2{margin-bottom:8px}.place p{margin:5px 0;color:#f4f1eb}.palette{display:flex;justify-content:center;gap:14px;margin:30px 0}.color{width:48px;height:48px;border-radius:50%;border:4px solid var(--paper);box-shadow:0 2px 10px rgba(0,0,0,.1)}.color:nth-child(1){background:#87927b}.color:nth-child(2){background:#c99f95}.color:nth-child(3){background:#d8c8b5}.color:nth-child(4){background:#4c554a}.rsvp{padding:100px 20px;background:var(--paper);border:1px solid var(--line)}.rsvp .monogram{margin-bottom:20px;color:var(--rose);font-size:42px;font-style:italic}.rsvp p{max-width:580px;margin:0 auto}.footer{padding:45px 0;text-align:center;color:#7c8178;font-style:italic}@media(max-width:680px){.hero{padding:14px 0}.card{min-height:calc(100svh - 28px);display:flex;flex-direction:column;justify-content:center}.grid{grid-template-columns:1fr}.section{padding:68px 0}.date{gap:10px}.date:before,.date:after{width:28px}}
  </style>
</head>
<body>
  <header class="hero wrap">
    <div class="card">
      <p class="eyebrow">Приглашение на свадьбу</p>
      <h1 class="names">Анна<span class="amp">и</span>Михаил</h1>
      <div class="date"><span>12 · 09 · 2026</span></div>
      <p class="intro">Дорогие родные и друзья! Совсем скоро состоится день, который станет началом нашей семейной истории. Мы будем счастливы разделить его с вами.</p>
      <a class="button" href="#details">Узнать подробности</a>
    </div>
  </header>
  <main>
    <section class="section wrap" id="details">
      <p class="eyebrow">Наш день</p>
      <h2>Программа</h2>
      <p class="subtitle">Пожалуйста, приезжайте немного заранее, чтобы мы успели обняться и сделать первые фотографии.</p>
      <div class="grid">
        <article class="item"><span class="time">15:30</span><h3>Сбор гостей</h3><p>Welcome-зона и лёгкие закуски</p></article>
        <article class="item"><span class="time">16:00</span><h3>Церемония</h3><p>Самые важные слова и обещания</p></article>
        <article class="item"><span class="time">17:00</span><h3>Ужин</h3><p>Праздник, музыка и танцы</p></article>
      </div>
    </section>
    <section class="place">
      <div class="wrap"><p class="eyebrow">Место встречи</p><h2>Усадьба «Лесная»</h2><p>Московская область, Цветочная улица, 12</p><p>Сбор гостей у главной террасы</p></div>
    </section>
    <section class="section wrap">
      <p class="eyebrow">Несколько деталей</p>
      <h2>Дресс-код</h2>
      <p class="subtitle">Будем рады видеть вас в спокойных природных оттенках. Главное — выбирайте образ, в котором вам красиво и комфортно.</p>
      <div class="palette" aria-label="Цветовая палитра"><span class="color"></span><span class="color"></span><span class="color"></span><span class="color"></span></div>
    </section>
    <section class="rsvp wrap" id="rsvp">
      <div class="monogram">А &amp; М</div>
      <p class="eyebrow">До встречи</p>
      <h2>Будем ждать вас</h2>
      <p>Пожалуйста, сообщите нам до 1 августа, сможете ли вы присоединиться. Ваш ответ поможет нам позаботиться о каждом госте.</p>
      <a class="button" href="#rsvp">Подтвердить участие</a>
    </section>
  </main>
  <footer class="footer wrap">С любовью, Анна и Михаил</footer>
</body>
</html>`

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
		if !validTelegramPath(r.URL.Path) {
			servePlaceholder(w)
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
		logger.Info("api request", "method", r.Method, "path", redactedPath(r.URL.Path), "remote", remoteIP(r.RemoteAddr), "duration_ms", time.Since(start).Milliseconds())
		proxy.ServeHTTP(w, r)
	}), nil
}

func servePlaceholder(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, placeholderHTML)
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
			if _, err := os.Stat(".env"); err != nil {
				fmt.Println("Сначала выберите пункт 1 и завершите настройку.")
				continue
			}
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
	askPort := func(label, def string) string {
		for {
			value := ask(label, def)
			if validPort(value) {
				return value
			}
			fmt.Println("Введите номер порта от 1 до 65535, например", def)
		}
	}
	var host string
	for {
		host = strings.TrimSuffix(ask("Публичный IP или домен", env("TGPROXY_PUBLIC_HOST", "example.com")), ".")
		if net.ParseIP(host) != nil || isDomain(host) {
			break
		}
		fmt.Println("Введите корректный публичный IP или домен, например tg.example.com")
	}
	https := false
	if isDomain(host) {
		https = answerYes(ask("Включить HTTPS с автоматическим сертификатом? (да/нет)", "да"))
	} else {
		fmt.Println("Для IP HTTPS не включён: укажите домен с DNS-записью на этот сервер.")
	}
	mtDefault, apiDefault, profile := "443", "8443", "http"
	if https {
		mtDefault, apiDefault, profile = "8443", "443", "https"
	}
	mtPort := askPort("Порт MTProto", mtDefault)
	apiPort := apiDefault
	if https {
		fmt.Println("Порт Bot API: 443 (автоматически для HTTPS)")
	} else {
		apiPort = askPort("Порт Bot API", apiDefault)
	}
	for mtPort == apiPort {
		fmt.Println("Порты MTProto и Bot API должны отличаться.")
		mtPort = askPort("Другой порт MTProto", "8443")
	}
	apiKey := randomHex(32)
	secret := generateSecret()
	content := fmt.Sprintf("TGPROXY_PUBLIC_HOST=%s\nTGPROXY_MTG_PORT=%s\nTGPROXY_API_PORT=%s\nTGPROXY_API_KEY=%s\nTGPROXY_MTG_SECRET=%s\nCOMPOSE_PROFILES=%s\n", host, mtPort, apiPort, apiKey, secret, profile)
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		return err
	}
	fmt.Println("Создан .env. Ключ Bot API:", apiKey)
	if https {
		fmt.Printf("HTTPS будет доступен по адресу https://%s/ после запуска.\n", host)
		fmt.Println("Убедитесь, что DNS уже указывает на сервер, а TCP-порты 80 и 443 открыты.")
	}
	return nil
}

func isDomain(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func answerYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "да", "д", "yes", "y", "1", "true":
		return true
	default:
		return false
	}
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
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
