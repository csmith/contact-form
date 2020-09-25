package main

import (
	"github.com/nelkinda/health-go"
	"net/http"
	"time"
)

type healthCheck struct {
	mail, captcha error
}

func (h *healthCheck) HealthChecks() map[string][]health.Checks {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	errorCheck := func(err error) health.Checks {
		if err == nil {
			return health.Checks{
				ComponentType: "component",
				Status:        health.Pass,
				Time:          now,
			}
		} else {
			return health.Checks{
				ComponentType: "component",
				Status:        health.Fail,
				Time:          now,
				Output:        err.Error(),
			}
		}
	}

	return map[string][]health.Checks{
		"mail":    {errorCheck(h.mail)},
		"captcha": {errorCheck(h.captcha)},
	}
}

func (*healthCheck) AuthorizeHealth(*http.Request) bool {
	return true
}

func (h *healthCheck) recordMailFailure(err error) {
	h.mail = err
}

func (h *healthCheck) recordMailSuccess() {
	h.mail = nil
}

func (h *healthCheck) recordCaptchaError(err error) {
	h.captcha = err
}

func (h *healthCheck) recordCaptchaSuccess() {
	h.captcha = nil
}
