package main

import (
	"github.com/dchest/captcha"
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
		log.Debug("Attempted to show captcha but session is in bad state. Redirecting to failure.")
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" || !captcha.Reload(captchaId) {
		log.Debug("Generating new captcha ID")
		captchaId = captcha.New()
		sessionManager.Put(req.Context(), captchaKey, captchaId)
	}

	_ = captchaTemplate.ExecuteTemplate(rw, "captcha.html", map[string]interface{}{
		"csrfField": "",
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
		log.Error("Unable to generate image captcha", "error", err)
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
		log.Error("Unable to generate audio captcha", "error", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	hc.recordCaptchaSuccess()
}

func handleSolve(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		log.Debug("Client tried to solve without a known captcha")
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	digits := req.Form.Get("captcha")
	if !captcha.VerifyString(captchaId, digits) {
		log.Debug("Client presented incorrect captcha solution")
		hc.recordCaptchaSuccess()
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
		return
	}

	log.Debug("Client presented correct captcha solution, sending mail")
	hc.recordCaptchaSuccess()
	if sendMail(sessionManager.PopString(req.Context(), replyToKey), sessionManager.PopString(req.Context(), bodyKey)) {
		rw.Header().Add("Location", "success")
		rw.WriteHeader(http.StatusSeeOther)
	} else {
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}
}
