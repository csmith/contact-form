package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jamiealquiza/envy"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"strings"
)

const (
	csrfFieldName = "csrf.Token"
	sessionName   = "contactform"
	bodyKey       = "body"
	replyToKey    = "replyTo"
	captchaKey    = "captchaId"
)

var (
	fromAddress, toAddress, subject        *string
	smtpServer, smtpUsername, smtpPassword *string
	csrfKey, sessionKey                    *string
	smtpPort, port                         *int
	enableCaptcha                          *bool
	store                                  *sessions.CookieStore
	formTemplate                           *template.Template
	captchaTemplate                        *template.Template
	successTemplate, failureTemplate       *template.Template
)

func sendMail(replyTo, message string) bool {
	auth := smtp.PlainAuth("", *smtpUsername, *smtpPassword, *smtpServer)
	body := fmt.Sprintf("To: %s\r\nSubject: %s\r\nReply-to: %s\r\nFrom: Online contact form <%s>\r\n\r\n%s\r\n", *toAddress, *subject, replyTo, *fromAddress, message)
	err := smtp.SendMail(fmt.Sprintf("%s:%d", *smtpServer, *smtpPort), auth, *fromAddress, []string{*toAddress}, []byte(body))
	if err != nil {
		log.Printf("Unable to send mail: %s", err)
		return false
	}
	return true
}

func handleSubmit(rw http.ResponseWriter, req *http.Request) {
	body := ""
	for k, v := range req.Form {
		if k != csrfFieldName {
			body += fmt.Sprintf("%s:\r\n%s\r\n\r\n", strings.ToUpper(k), v[0])
		}
	}

	replyTo := req.Form.Get("from")
	replyTo = strings.ReplaceAll(replyTo, "\n", "")
	replyTo = strings.ReplaceAll(replyTo, "\r", "")

	if *enableCaptcha {
		beginCaptcha(rw, req, body, replyTo)
	} else if sendMail(replyTo, body) {
		rw.Header().Add("Location", "success")
		rw.WriteHeader(http.StatusSeeOther)
	} else {
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
		csrf.TemplateTag: csrf.TemplateField(req),
		"params":         params,
	})
}

func showSuccess(rw http.ResponseWriter, req *http.Request) {
	_ = successTemplate.ExecuteTemplate(rw, "success.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(req),
	})
}

func showFailure(rw http.ResponseWriter, req *http.Request) {
	_ = failureTemplate.ExecuteTemplate(rw, "failure.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(req),
	})
}

func randomKey() string {
	var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 32)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
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

func main() {
	fromAddress = flag.String("from", "", "address to send e-mail from")
	toAddress = flag.String("to", "", "address to send e-mail to")
	subject = flag.String("subject", "Contact form submission", "e-mail subject")
	smtpServer = flag.String("smtp-host", "", "SMTP server to connect to")
	smtpPort = flag.Int("smtp-port", 25, "port to use when connecting to the SMTP server")
	smtpUsername = flag.String("smtp-user", "", "username to supply to the SMTP server")
	smtpPassword = flag.String("smtp-pass", "", "password to supply to the SMTP server")
	csrfKey = flag.String("crsf-key", "", "CRSF key to use")
	sessionKey = flag.String("session-key", "", "Session key to use (for captcha support)")
	enableCaptcha = flag.Bool("enable-captcha", false, "Whether to require captchas to be completed")
	port = flag.Int("port", 8080, "port to listen on for connections")

	envy.Parse("CONTACT")
	flag.Parse()

	checkFlag(*fromAddress, "from address")
	checkFlag(*toAddress, "to address")
	checkFlag(*smtpServer, "SMTP server")
	checkFlag(*smtpUsername, "SMTP username")
	checkFlag(*smtpPassword, "SMTP password")

	if len(*csrfKey) != 32 {
		newKey := randomKey()
		csrfKey = &newKey
	}

	if len(*sessionKey) != 32 {
		newKey := randomKey()
		sessionKey = &newKey
	}

	store = sessions.NewCookieStore([]byte(*sessionKey))
	store.Options =  &sessions.Options{
		MaxAge:   0,
		Secure:   false, //true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	formTemplate = loadTemplate("form.html")
	captchaTemplate = loadTemplate("captcha.html")
	successTemplate = loadTemplate("success.html")
	failureTemplate = loadTemplate("failure.html")

	r := mux.NewRouter()
	r.HandleFunc("/", showForm).Methods("GET")
	r.HandleFunc("/success", showSuccess).Methods("GET")
	r.HandleFunc("/failure", showFailure).Methods("GET")
	r.HandleFunc("/submit", handleSubmit).Methods("POST")

	// Captcha endpoints
	r.HandleFunc("/captcha", showCaptcha).Methods("GET")
	r.HandleFunc("/captcha.png", writeCaptchaImage).Methods("GET")
	r.HandleFunc("/captcha.wav", writeCaptchaAudio).Methods("GET")
	r.HandleFunc("/solve", handleSolve).Methods("POST")

	// If developing locally, you'll need to pass csrf.Secure(false) as an argument below.
	CSRF := csrf.Protect([]byte(*csrfKey), csrf.FieldName(csrfFieldName), csrf.Secure(false))
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), CSRF(r))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to listen on port %d: %s\n", *port, err.Error())
	}
}
