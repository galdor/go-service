package shttp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/galdor/go-service/pkg/influx"
	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/utils"
)

var assetCacheBustingRE = regexp.MustCompile(
	`^(.+)\.(?:[a-z0-9]+)\.([^.]+)$`)

type Handler struct {
	Server *Server
	Log    *log.Logger

	Method      string
	PathPattern string
	RouteId     string // based on the method and path pattern

	Request        *http.Request
	Query          url.Values
	ResponseWriter http.ResponseWriter

	ClientAddress string // optional
	RequestId     string // optional

	start         time.Time
	pathVariables map[string]string
	errorCode     string
}

func RouteId(method, pathPattern string) string {
	if method == "" || pathPattern == "" {
		return ""
	}

	return pathPattern + " " + method
}

func (h *Handler) PathVariable(name string) string {
	value, found := h.pathVariables[name]
	if !found {
		utils.Panicf("unknown path variable %q", name)
	}

	return value
}

func (h *Handler) HasQueryParameter(name string) bool {
	return h.Query.Has(name)
}

func (h *Handler) QueryParameter(name string) string {
	return h.Query.Get(name)
}

func (h *Handler) Reply(status int, r io.Reader) {
	h.ResponseWriter.WriteHeader(status)

	if r != nil {
		if _, err := io.Copy(h.ResponseWriter, r); err != nil {
			h.Log.Error("cannot write response: %v", err)
			return
		}
	}
}

func (h *Handler) ReplyEmpty(status int) {
	h.Reply(status, nil)
}

func (h *Handler) ReplyRedirect(status int, uri string) {
	header := h.ResponseWriter.Header()
	header.Set("Location", uri)

	h.Reply(status, nil)
}

func (h *Handler) ReplyText(status int, body string) {
	header := h.ResponseWriter.Header()
	header.Set("Content-Type", "text/plain; charset=UTF-8")

	h.Reply(status, strings.NewReader(body))
}

func (h *Handler) ReplyError(status int, code, format string, args ...interface{}) {
	h.ReplyErrorData(status, code, nil, format, args...)
}

func (h *Handler) ReplyErrorData(status int, code string, data ErrorData, format string, args ...interface{}) {
	h.errorCode = code
	h.Server.errorHandler(h, status, code, fmt.Sprintf(format, args...), data)
}

func (h *Handler) ReplyInternalError(status int, format string, args ...interface{}) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
	h.Log.Error("internal error: %s", msg)

	if h.Server.Cfg.HideInternalErrors {
		msg = "internal error"
	}

	h.ReplyError(status, "internal_error", msg)
}

func (h *Handler) ReplyNotImplemented(feature string) {
	h.ReplyError(501, "not_implemented", "%s not implemented", feature)
}

func (h *Handler) ReplyFile(filePath string) {
	filePath = rewriteAssetPath(filePath)

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			h.ReplyError(404, "not_found", "file not found")
			return
		}

		h.ReplyInternalError(500, "cannot stat %q: %v", filePath, err)
		return
	}

	if !info.Mode().IsRegular() {
		h.ReplyError(400, "%q is not a regular file", filePath)
		return
	}

	modTime := info.ModTime()

	body, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			h.ReplyError(404, "not_found", "file not found")
			return
		}

		h.ReplyInternalError(500, "cannot open %q: %v", filePath, err)
		return
	}
	defer body.Close()

	http.ServeContent(h.ResponseWriter, h.Request, filePath, modTime, body)
}

func rewriteAssetPath(path string) string {
	matches := assetCacheBustingRE.FindAllStringSubmatch(path, -1)
	if len(matches) < 1 {
		return path
	}

	groups := matches[0][1:]
	return groups[0] + "." + groups[1]
}

func (h *Handler) logRequest() {
	req := h.Request
	w := h.ResponseWriter.(*ResponseWriter)

	if !h.Server.Cfg.LogSuccessfulRequests {
		if w.Status >= 100 && w.Status < 400 {
			return
		}
	}

	reqTime := time.Since(h.start)

	data := log.Data{
		"event":        "http.incomingRequest",
		"time":         reqTime.Microseconds(),
		"responseSize": w.ResponseBodySize,
	}

	if h.ClientAddress != "" {
		data["address"] = h.ClientAddress
	}

	if h.RequestId != "" {
		data["requestId"] = h.RequestId
	}

	if h.errorCode != "" {
		data["error"] = h.errorCode
	}

	statusString := "-"
	if w.Status != 0 {
		statusString = strconv.Itoa(w.Status)
		data["status"] = w.Status
	}

	h.Log.InfoData(data, "%s %s %s %s",
		req.Method, req.URL.Path, statusString,
		utils.FormatSeconds(reqTime.Seconds(), 1))
}

func (h *Handler) sendInfluxPoints() {
	if h.Server.Cfg.InfluxClient == nil {
		return
	}

	w := h.ResponseWriter.(*ResponseWriter)

	now := time.Now()
	reqTime := time.Since(h.start)

	tags := influx.Tags{
		"server": h.Server.Cfg.Name,
	}

	if h.RouteId != "" {
		tags["route"] = h.RouteId
	}

	fields := influx.Fields{
		"time":         reqTime.Microseconds(),
		"status":       w.Status,
		"responseSize": w.ResponseBodySize,
	}

	if w.Status != 0 {
		fields["status"] = w.Status
	}

	point := influx.NewPointWithTimestamp("incomingHTTPRequests",
		tags, fields, now)

	h.Server.Cfg.InfluxClient.EnqueuePoint(point)
}
