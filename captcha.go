package main

import (
	"github.com/dchest/captcha"
	"github.com/gorilla/csrf"
	"log"
	"net/http"
)

func beginCaptcha(rw http.ResponseWriter, req *http.Request, body string, replyTo string) {
	session, err := store.New(req, sessionName)
	if err != nil {
		log.Printf("Unable to get session: %s", err.Error())
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	session.Values[bodyKey] = body
	session.Values[replyToKey] = replyTo

	err = session.Save(req, rw)
	if err != nil {
		log.Printf("Unable to save session: %s", err.Error())
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	rw.Header().Add("Location", "captcha")
	rw.WriteHeader(http.StatusSeeOther)
}

func showCaptcha(rw http.ResponseWriter, req *http.Request) {
	session, err := store.Get(req, sessionName)
	if err != nil {
		log.Printf("Unable to get session: %s", err.Error())
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	captchaId, ok := session.Values[captchaKey]
	if !ok || !captcha.Reload(captchaId.(string)) {
		captchaId = captcha.New()
		session.Values[captchaKey] = captchaId

		if err := session.Save(req, rw); err != nil {
			log.Printf("Unable to save session: %s", err.Error())
			rw.Header().Add("Location", "failure")
			rw.WriteHeader(http.StatusSeeOther)
			return
		}
	}

	_ = captchaTemplate.ExecuteTemplate(rw, "captcha.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(req),
	})
}

func writeCaptchaImage(rw http.ResponseWriter, req *http.Request) {
	captchaId, ok := findCaptcha(req)
	if !ok {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "image/png")
	if err := captcha.WriteImage(rw, captchaId, captcha.StdWidth, captcha.StdHeight); err != nil {
		log.Printf("Unable to generate image captcha: %s", err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func writeCaptchaAudio(rw http.ResponseWriter, req *http.Request) {
	captchaId, ok := findCaptcha(req)
	if !ok {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "audio/wav")
	rw.Header().Set("Content-Disposition", "attachment")
	if err := captcha.WriteAudio(rw, captchaId, "en"); err != nil {
		log.Printf("Unable to generate audio captcha: %s", err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleSolve(rw http.ResponseWriter, req *http.Request) {
	captchaId, ok := findCaptcha(req)
	if !ok {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	digits := req.Form.Get("captcha")
	if !captcha.VerifyString(captchaId, digits) {
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	session, err := store.Get(req, sessionName)
	if err != nil {
		log.Printf("Unable to get session: %s", err.Error())
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	if sendMail(session.Values[replyToKey].(string), session.Values[bodyKey].(string)) {
		rw.Header().Add("Location", "success")
		rw.WriteHeader(http.StatusSeeOther)
	} else {
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}
}

func findCaptcha(req *http.Request) (string, bool) {
	session, err := store.Get(req, sessionName)
	if err != nil {
		log.Printf("Unable to get session: %s", err.Error())
		return "", false
	}

	captchaId, ok := session.Values[captchaKey]
	if !ok {
		return "", false
	}

	return captchaId.(string), true
}
