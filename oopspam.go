package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

const (
	oopspamUrl       = "https://api.oopspam.com/v1/spamdetection"
	oopspamThreshold = 3.0
)

type OopspamRequest struct {
	Content  string `json:"content"`
	SenderIp string `json:"senderIP"`
	Email    string `json:"email"`

	BlockTempEmail bool `json:"blockTempEmail"`
	BlockVpn       bool `json:"blockVPN"`
	BlockDc        bool `json:"blockDC"`
	CheckForLength bool `json:"checkForLength"`
	UrlFriendly    bool `json:"urlFriendly"`
}

type OopspamResponse struct {
	Score   int                    `json:"score"`
	Details map[string]interface{} `json:"details"`
}

func checkSpam(rw http.ResponseWriter, req *http.Request, body string, replyTo string) {
	payload, err := json.Marshal(&OopspamRequest{
		Content:        body,
		SenderIp:       senderIp(req),
		Email:          replyTo,
		BlockTempEmail: *oopspamBlockTempEmail,
		BlockVpn:       *oopspamBlockVpn,
		BlockDc:        *oopspamBlockDc,
		CheckForLength: *oopspamCheckForLength,
		UrlFriendly:    *oopspamUrlFriendly,
	})
	if err != nil {
		log.Error("Failed to marshal oopspam request", "replyTo", replyTo, "err", err)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}

	spamReq, err := http.NewRequest(http.MethodPost, oopspamUrl, bytes.NewReader(payload))
	if err != nil {
		log.Error("Failed to construct oopspam request", "replyTo", replyTo, "err", err)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}

	spamReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	spamReq.Header.Set("X-Api-Key", *oopspamApiKey)

	spamRes, err := http.DefaultClient.Do(spamReq)
	if err != nil {
		log.Error("Failed to send oopspam request", "replyTo", replyTo, "err", err)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}
	if spamRes.StatusCode >= 400 {
		log.Error("Failed to send oopspam request", "replyTo", replyTo, "statusCode", spamRes.StatusCode)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}

	defer spamRes.Body.Close()
	spamResult := &OopspamResponse{}
	if err := json.NewDecoder(spamRes.Body).Decode(spamResult); err != nil {
		log.Error("Failed to unmarshal oopspam response", "replyTo", replyTo, "err", err)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}

	log.Info("Spam check result from oopspam", "score", spamResult.Score, "details", spamResult.Details, "replyTo", replyTo)
	if spamResult.Score >= oopspamThreshold {
		handleSpamFailure(rw, req, body, replyTo, *oopspamSpamHandler)
	} else {
		trySendMail(rw, replyTo, body)
	}
}

func handleSpamFailure(rw http.ResponseWriter, req *http.Request, body string, replyTo string, handler string) {
	switch handler {
	case "deny":
		log.Info("Failing request due to oopspam policy", "replyTo", replyTo)
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	case "captcha":
		log.Info("Starting captcha due to oopspam policy", "replyTo", replyTo)
		beginCaptcha(rw, req, body, replyTo)
	case "allow":
		log.Info("Sending message due to oopspam policy", "replyTo", replyTo)
		trySendMail(rw, body, replyTo)
	default:
		log.Error("Unknown oopspam handler behaviour", "replyTo", replyTo, "handler", handler)
		rw.Header().Add("Location", "failure")
		rw.WriteHeader(http.StatusSeeOther)
	}
}

func senderIp(req *http.Request) string {
	address, _, _ := net.SplitHostPort(req.RemoteAddr)
	if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		address = strings.Split(forwardedFor, ", ")[0]
	}
	return address
}
