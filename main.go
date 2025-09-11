package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"filippo.io/csrf"
	"github.com/alexedwards/scs/boltstore"
	"github.com/alexedwards/scs/v2"
	"github.com/csmith/envflag/v2"
	"github.com/csmith/slogflags"
	"github.com/nelkinda/health-go"
	"go.etcd.io/bbolt"
)

const (
	sessionName = "contactform"
	bodyKey     = "body"
	replyToKey  = "replyTo"
	captchaKey  = "captchaId"
)

var (
	fromAddress           = flag.String("from", "", "address to send e-mail from")
	toAddress             = flag.String("to", "", "address to send e-mail to")
	subject               = flag.String("subject", "Contact form submission", "e-mail subject")
	smtpServer            = flag.String("smtp-host", "", "SMTP server to connect to")
	smtpPort              = flag.Int("smtp-port", 25, "port to use when connecting to the SMTP server")
	smtpUsername          = flag.String("smtp-user", "", "username to supply to the SMTP server")
	smtpPassword          = flag.String("smtp-pass", "", "password to supply to the SMTP server")
	sessionPath           = flag.String("session-path", "./sessions.db", "Path to persist session information")
	enableCaptcha         = flag.Bool("enable-captcha", false, "Whether to require captchas to be completed")
	oopspamApiKey         = flag.String("oopspam-apikey", "", "API key to use for OOPSpam (disabled if not set)")
	oopspamErrorHandler   = flag.String("oopspam-error-handler", "deny", "What to do if OOPSpam check errors (captcha, allow, deny)")
	oopspamSpamHandler    = flag.String("oopspam-spam-handler", "deny", "What to do if OOPSpam detects spam (captcha, allow, deny)")
	oopspamBlockTempEmail = flag.Bool("oopspam-block-temp-email", false, "Whether to block temporary email addresses")
	oopspamBlockVpn       = flag.Bool("oopspam-block-vpn", false, "Whether to block messages routed via VPN providers")
	oopspamBlockDc        = flag.Bool("oopspam-block-dc", false, "Whether to block messages routed from data centre IP ranges")
	oopspamCheckForLength = flag.Bool("oopspam-check-for-length", true, "Whether to check minimum message length")
	oopspamUrlFriendly    = flag.Bool("oopspam-url-friendly", false, "Whether to reduce the impact of links on spam score")
	enableHealthCheck     = flag.Bool("enable-health-check", false, "Whether to expose health checks at /_health")
	port                  = flag.Int("port", 8080, "port to listen on for connections")
	csrfTrustedOrigins    = flag.String("csrf-trusted-origins", "", "Comma-separated list of trusted origins to bypass CSRF checks")

	formTemplate    *template.Template
	captchaTemplate *template.Template
	successTemplate *template.Template
	failureTemplate *template.Template

	sessionManager *scs.SessionManager

	hc  = &healthCheck{}
	log *slog.Logger
)

func main() {
	envflag.Parse(envflag.WithPrefix("CONTACT_"))
	log = slogflags.Logger()

	checkFlag(*fromAddress, "from address")
	checkFlag(*toAddress, "to address")
	checkFlag(*smtpServer, "SMTP server")
	checkFlag(*smtpUsername, "SMTP username")
	checkFlag(*smtpPassword, "SMTP password")

	db, err := bbolt.Open(*sessionPath, 0600, nil)
	if err != nil {
		log.Error("Unable to open session database", "error", err, "path", *sessionPath)
		os.Exit(1)
	}
	defer db.Close()

	sessionManager = scs.New()
	sessionManager.Store = boltstore.NewWithCleanupInterval(db, time.Hour)
	sessionManager.Cookie.Name = sessionName
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Persist = false
	sessionManager.Cookie.Secure = true
	sessionManager.Cookie.SameSite = http.SameSiteStrictMode

	formTemplate = loadTemplate("templates/form.html")
	captchaTemplate = loadTemplate("templates/captcha.html")
	successTemplate = loadTemplate("templates/success.html")
	failureTemplate = loadTemplate("templates/failure.html")

	r := http.NewServeMux()
	r.HandleFunc("GET /", showForm)
	r.HandleFunc("GET /success", showSuccess)
	r.HandleFunc("GET /failure", showFailure)
	r.HandleFunc("POST /submit", handleSubmit)

	// Static files (with no index)
	r.Handle("GET /static/{$}", http.NotFoundHandler())
	r.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Captcha endpoints
	r.HandleFunc("GET /captcha", showCaptcha)
	r.HandleFunc("GET /captcha.png", writeCaptchaImage)
	r.HandleFunc("GET /captcha.wav", writeCaptchaAudio)
	r.HandleFunc("POST /solve", handleSolve)

	// Health checks
	if *enableHealthCheck {
		log.Debug("Registering health check handler")
		h := health.New(health.Health{Version: "1"}, hc)
		r.HandleFunc("GET /_health", h.Handler)
	}

	protection := csrf.New()
	trustedOrigins := strings.Split(*csrfTrustedOrigins, ",")
	for i := range trustedOrigins {
		if trustedOrigins[i] != "" {
			log.Debug("Registering trusted origin", "origin", trustedOrigins[i])
			if err := protection.AddTrustedOrigin(trustedOrigins[i]); err != nil {
				log.Error("Failed to add trusted CSRF origin", "error", err, "origin", trustedOrigins[i])
				os.Exit(1)
			}
		}
	}

	version := "unknown"
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		version = buildInfo.Main.Version
	}
	log.Info("Starting contact form server...", "port", *port, "version", version)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: sessionManager.LoadAndSave(protection.Handler(r)),
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error("Unable to listen on port", "port", *port, "error", err)
		}
		log.Info("Contact form server stopped")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to shut down HTTP server", "error", err)
	}
}

func sendMail(replyTo, message string) bool {
	auth := smtp.PlainAuth("", *smtpUsername, *smtpPassword, *smtpServer)
	body := fmt.Sprintf("To: %s\r\nSubject: %s\r\nReply-to: %s\r\nFrom: Online contact form <%s>\r\n\r\n%s\r\n", *toAddress, *subject, replyTo, *fromAddress, message)
	log.Debug("Sending e-mail message", "from", *fromAddress, "to", *toAddress, "subject", *subject, "replyTo", replyTo)
	err := smtp.SendMail(fmt.Sprintf("%s:%d", *smtpServer, *smtpPort), auth, *fromAddress, []string{*toAddress}, []byte(body))
	if err != nil {
		log.Error("Unable to send e-mail", "error", err)
		hc.recordMailFailure(err)
		return false
	}
	hc.recordMailSuccess()
	return true
}

func handleSubmit(rw http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		log.Warn("Unable to parse form", "error", err)
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}

	body := ""
	for k, v := range req.Form {
		body += fmt.Sprintf("%s:\r\n%s\r\n\r\n", strings.ToUpper(k), v[0])
	}

	replyTo := req.Form.Get("from")
	replyTo = strings.ReplaceAll(replyTo, "\n", "")
	replyTo = strings.ReplaceAll(replyTo, "\r", "")

	if *oopspamApiKey != "" {
		log.Debug("Form submitted, checking for spam", "replyTo", replyTo)
		checkSpam(rw, req, body, replyTo)
	} else if *enableCaptcha {
		log.Debug("Form submitted, presenting captcha", "replyTo", replyTo)
		beginCaptcha(rw, req, body, replyTo)
	} else {
		trySendMail(rw, replyTo, body)
	}
}

func trySendMail(rw http.ResponseWriter, replyTo, body string) {
	if sendMail(replyTo, body) {
		log.Debug("Form submitted successfully, redirecting to success handler", "replyTo", replyTo)
		rw.Header().Add("Location", "success")
		rw.WriteHeader(http.StatusSeeOther)
	} else {
		log.Debug("Form submitted with error, redirecting to failure handler", "replyTo", replyTo)
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}
}

func showForm(rw http.ResponseWriter, req *http.Request) {
	params := make(map[string]string)

	for k, vs := range req.URL.Query() {
		if len(vs) == 1 {
			params[k] = vs[0]
		}
	}

	_ = formTemplate.ExecuteTemplate(rw, "form.html", map[string]interface{}{
		"params": params,
	})
}

func showSuccess(rw http.ResponseWriter, req *http.Request) {
	_ = successTemplate.ExecuteTemplate(rw, "success.html", map[string]interface{}{
		"csrfField": "",
	})
}

func showFailure(rw http.ResponseWriter, req *http.Request) {
	_ = failureTemplate.ExecuteTemplate(rw, "failure.html", map[string]interface{}{
		"csrfField": "",
	})
}

func checkFlag(value string, name string) {
	if len(value) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "No %s specified\n", name)
		flag.Usage()
		os.Exit(1)
	}
}

func loadTemplate(file string) (result *template.Template) {
	var err error
	result, err = template.ParseFiles(file)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to load %s: %s\n", file, err.Error())
		os.Exit(1)
	}
	return
}
