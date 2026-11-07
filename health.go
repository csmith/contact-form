package main

import (
	"encoding/json"
	"net/http"
)

type healthCheck struct {
	mail, captcha error
}

func (h *healthCheck) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	type response struct {
		Status string            `json:"status"`
		Errors map[string]string `json:"errors,omitempty"`
	}

	res := response{Status: "pass", Errors: map[string]string{}}
	for name, err := range map[string]error{"mail": h.mail, "captcha": h.captcha} {
		if err != nil {
			res.Errors[name] = err.Error()
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	if len(res.Errors) > 0 {
		res.Status = "fail"
		rw.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(rw).Encode(res); err != nil {
		log.Warn("Unable to write health check response", "error", err)
	}
}
