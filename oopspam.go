package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

const (
	oopspamUrl       = "https://api.oopspam.com/v1/spamdetection"
	oopspamThreshold = 3.0
)

func checkSpam(rw http.ResponseWriter, req *http.Request, body string, replyTo string) {
	score, err := oopspamScore(req, body, replyTo)
	if err != nil {
		log.Error("Failed to check message with oopspam", "replyTo", replyTo, "err", err)
		handleSpamFailure(rw, req, body, replyTo, *oopspamErrorHandler)
		return
	}

	if score >= oopspamThreshold {
		handleSpamFailure(rw, req, body, replyTo, *oopspamSpamHandler)
	} else {
		trySendMail(rw, replyTo, body)
	}
}

func oopspamScore(req *http.Request, body string, replyTo string) (int, error) {
	type oopspamRequest struct {
		Content  string `json:"content"`
		SenderIp string `json:"senderIP"`
		Email    string `json:"email"`

		BlockTempEmail bool `json:"blockTempEmail"`
		BlockVpn       bool `json:"blockVPN"`
		BlockDc        bool `json:"blockDC"`
		CheckForLength bool `json:"checkForLength"`
		UrlFriendly    bool `json:"urlFriendly"`
	}

	type oopspamResponse struct {
		Score   int            `json:"score"`
		Details map[string]any `json:"details"`
	}

	payload, err := json.Marshal(
		&oopspamRequest{
			Content:        body,
			SenderIp:       senderIp(req),
			Email:          replyTo,
			BlockTempEmail: *oopspamBlockTempEmail,
			BlockVpn:       *oopspamBlockVpn,
			BlockDc:        *oopspamBlockDc,
			CheckForLength: *oopspamCheckForLength,
			UrlFriendly:    *oopspamUrlFriendly,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	spamReq, err := http.NewRequestWithContext(
		req.Context(),
		http.MethodPost,
		oopspamUrl,
		bytes.NewReader(payload),
	)
	if err != nil {
		return 0, fmt.Errorf("construct request: %w", err)
	}

	spamReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	spamReq.Header.Set("X-Api-Key", *oopspamApiKey)

	spamRes, err := http.DefaultClient.Do(spamReq)
	if err != nil {
		return 0, fmt.Errorf("send request: %w", err)
	}
	defer spamRes.Body.Close()

	if spamRes.StatusCode >= 400 {
		return 0, fmt.Errorf("unexpected status code %d", spamRes.StatusCode)
	}

	spamResult := &oopspamResponse{}
	if err := json.NewDecoder(spamRes.Body).Decode(spamResult); err != nil {
		return 0, fmt.Errorf("unmarshal response: %w", err)
	}

	log.Info(
		"Spam check result from oopspam",
		"score",
		spamResult.Score,
		"details",
		spamResult.Details,
		"replyTo",
		replyTo,
	)
	return spamResult.Score, nil
}

func handleSpamFailure(rw http.ResponseWriter, req *http.Request, body string, replyTo string, handler string) {
	switch handler {
	case "deny":
		log.Info("Failing request due to oopspam policy", "replyTo", replyTo)
		redirect(rw, "failure")
	case "captcha":
		log.Info("Starting captcha due to oopspam policy", "replyTo", replyTo)
		beginCaptcha(rw, req, body, replyTo)
	case "allow":
		log.Info("Sending message due to oopspam policy", "replyTo", replyTo)
		trySendMail(rw, replyTo, body)
	default:
		log.Error("Unknown oopspam handler behaviour", "replyTo", replyTo, "handler", handler)
		redirect(rw, "failure")
	}
}

func senderIp(req *http.Request) string {
	address, _, _ := net.SplitHostPort(req.RemoteAddr)
	if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		address = strings.Split(forwardedFor, ", ")[0]
	}
	return address
}
