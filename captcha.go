package main

import (
	"github.com/dchest/captcha"
	"github.com/gorilla/csrf"
	"log"
	"net/http"
)

func beginCaptcha(rw http.ResponseWriter, req *http.Request, body string, replyTo string) {
	sessionManager.Put(req.Context(), bodyKey, body)
	sessionManager.Put(req.Context(), replyToKey, replyTo)
	rw.Header().Add("Location", "captcha")
	rw.WriteHeader(http.StatusSeeOther)
}

func showCaptcha(rw http.ResponseWriter, req *http.Request) {
	if !sessionManager.Exists(req.Context(), bodyKey) || !sessionManager.Exists(req.Context(), replyToKey) {
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" || !captcha.Reload(captchaId) {
		captchaId = captcha.New()
		sessionManager.Put(req.Context(), captchaKey, captchaId)
	}

	_ = captchaTemplate.ExecuteTemplate(rw, "captcha.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(req),
	})
}

func writeCaptchaImage(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "image/png")
	if err := captcha.WriteImage(rw, captchaId, captcha.StdWidth, captcha.StdHeight); err != nil {
		hc.recordCaptchaError(err)
		log.Printf("Unable to generate image captcha: %s", err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	hc.recordCaptchaSuccess()
}

func writeCaptchaAudio(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "audio/wav")
	rw.Header().Set("Content-Disposition", "attachment")
	if err := captcha.WriteAudio(rw, captchaId, "en"); err != nil {
		hc.recordCaptchaError(err)
		log.Printf("Unable to generate audio captcha: %s", err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	hc.recordCaptchaSuccess()
}

func handleSolve(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	digits := req.Form.Get("captcha")
	if !captcha.VerifyString(captchaId, digits) {
		hc.recordCaptchaSuccess()
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	hc.recordCaptchaSuccess()
	if sendMail(sessionManager.PopString(req.Context(), replyToKey), sessionManager.PopString(req.Context(), bodyKey)) {
		rw.Header().Add("Location", "success")
		rw.WriteHeader(http.StatusSeeOther)
	} else {
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}
}
