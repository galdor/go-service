package shttp

import (
	"net/http"
	"strconv"
	"time"

	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/utils"
)

type RoundTripper struct {
	Cfg *ClientCfg
	Log *log.Logger

	http.RoundTripper
}

func NewRoundTripper(rt http.RoundTripper, cfg *ClientCfg) *RoundTripper {
	return &RoundTripper{
		Cfg: cfg,
		Log: cfg.Log,

		RoundTripper: rt,
	}
}

func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	rt.finalizeReq(req)

	res, err := rt.RoundTripper.RoundTrip(req)

	if err == nil && rt.Cfg.LogRequests {
		rt.logRequest(req, res, time.Since(start).Seconds())
	}

	return res, err
}

func (rt *RoundTripper) finalizeReq(req *http.Request) {
	for name, values := range rt.Cfg.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
}

func (rt *RoundTripper) logRequest(req *http.Request, res *http.Response, seconds float64) {
	var statusString string
	if res == nil {
		statusString = "-"
	} else {
		statusString = strconv.Itoa(res.StatusCode)
	}

	rt.Log.Info("%s %s %s %s", req.Method, req.URL.String(), statusString,
		utils.FormatSeconds(seconds, 1))
}
