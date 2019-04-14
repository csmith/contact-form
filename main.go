package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
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
)

var (
	fromAddress, toAddress, subject, smtpServer, smtpUsername, smtpPassword, csrfKey *string
	smtpPort, port                                                                   *int
	formTemplate, successTemplate, failureTemplate                                   *template.Template
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

func handleForm(rw http.ResponseWriter, req *http.Request) {
	body := ""
	for k, v := range req.Form {
		if k != csrfFieldName {
			body += fmt.Sprintf("%s:\r\n%s\r\n\r\n", strings.ToUpper(k), v[0])
		}
	}
	if sendMail(req.Form.Get("from"), body) {
		rw.Header().Add("Location", "success")
	} else {
		rw.Header().Add("Location", "failure")
	}
	rw.WriteHeader(http.StatusTemporaryRedirect)
}

func showForm(rw http.ResponseWriter, req *http.Request) {
	_ = formTemplate.ExecuteTemplate(rw, "form.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(req),
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
	port = flag.Int("port", 8080, "port to listen on for connections")
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

	formTemplate = loadTemplate("form.html")
	successTemplate = loadTemplate("success.html")
	failureTemplate = loadTemplate("failure.html")

	r := mux.NewRouter()
	r.HandleFunc("/", showForm).Methods("GET")
	r.HandleFunc("/success", showSuccess).Methods("GET")
	r.HandleFunc("/failure", showFailure).Methods("GET")
	r.HandleFunc("/submit", handleForm).Methods("POST")

	CSRF := csrf.Protect([]byte(*csrfKey), csrf.FieldName(csrfFieldName))
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), CSRF(r))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to listen on port %d: %s\n", *port, err.Error())
	}
}
