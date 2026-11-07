package main

import (
	"errors"
	"net/http"

	"github.com/dchest/captcha"
)

func beginCaptcha(rw http.ResponseWriter, req *http.Request, body string, replyTo string) {
	sessionManager.Put(req.Context(), bodyKey, body)
	sessionManager.Put(req.Context(), replyToKey, replyTo)
	redirect(rw, "captcha")
}

func showCaptcha(rw http.ResponseWriter, req *http.Request) {
	if !sessionManager.Exists(req.Context(), bodyKey) || !sessionManager.Exists(req.Context(), replyToKey) {
		log.Debug("Attempted to show captcha but session is in bad state. Redirecting to failure.")
		redirect(rw, "failure")
		return
	}

	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" || !captcha.Reload(captchaId) {
		captchaId = captcha.New()
		log.Debug("Generated new captcha ID", "id", captchaId)
		sessionManager.Put(req.Context(), captchaKey, captchaId)
	}

	_ = captchaTemplate.ExecuteTemplate(
		rw,
		"captcha.html",
		nil,
	)
}

func writeCaptchaImage(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "image/png")
	if err := captcha.WriteImage(rw, captchaId, captcha.StdWidth, captcha.StdHeight); errors.Is(err, captcha.ErrNotFound) {
		log.Debug("Client requested image for unknown captcha", "id", captchaId)
		rw.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		hc.captcha = err
		log.Error("Unable to generate image captcha", "error", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	hc.captcha = nil
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
	if err := captcha.WriteAudio(rw, captchaId, "en"); errors.Is(err, captcha.ErrNotFound) {
		log.Debug("Client requested audio for unknown captcha", "id", captchaId)
		rw.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		hc.captcha = err
		log.Error("Unable to generate audio captcha", "error", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	hc.captcha = nil
}

func handleSolve(rw http.ResponseWriter, req *http.Request) {
	captchaId := sessionManager.GetString(req.Context(), captchaKey)
	if captchaId == "" {
		log.Debug("Client tried to solve without a known captcha")
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	req.Body = http.MaxBytesReader(rw, req.Body, maxRequestBytes)
	if err := req.ParseForm(); err != nil {
		log.Warn("Unable to parse form", "error", err)
		redirect(rw, "failure")
		return
	}

	// Verifying removes the captcha from the store whatever the outcome, so
	// the session's ID is no longer usable.
	solved := captcha.VerifyString(captchaId, req.Form.Get("captcha"))
	sessionManager.Remove(req.Context(), captchaKey)
	hc.captcha = nil

	if !solved {
		log.Debug("Client presented incorrect captcha solution", "id", captchaId)
		redirect(rw, "failure")
		return
	}

	log.Debug("Client presented correct captcha solution, sending mail", "id", captchaId)
	trySendMail(rw, sessionManager.PopString(req.Context(), replyToKey), sessionManager.PopString(req.Context(), bodyKey))
}
