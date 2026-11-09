package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type healthCheck struct {
	mail, captcha error
}

// ServeHTTP responds in the format described by draft-inadarei-api-health-check.
func (h *healthCheck) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	type check struct {
		ComponentType string `json:"componentType"`
		Status        string `json:"status"`
		Time          string `json:"time"`
		Output        string `json:"output,omitempty"`
	}

	type response struct {
		Status  string             `json:"status"`
		Version string             `json:"version"`
		Checks  map[string][]check `json:"checks"`
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res := response{Status: "pass", Version: "1", Checks: map[string][]check{}}
	for name, err := range map[string]error{"mail": h.mail, "captcha": h.captcha} {
		c := check{ComponentType: "component", Status: "pass", Time: now}
		if err != nil {
			c.Status = "fail"
			c.Output = err.Error()
			res.Status = "fail"
		}
		res.Checks[name] = []check{c}
	}

	rw.Header().Set("Content-Type", "application/health+json")
	if res.Status == "fail" {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(rw).Encode(res); err != nil {
		log.Warn("Unable to write health check response", "error", err)
	}
}
