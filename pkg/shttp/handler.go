package shttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/galdor/go-ejson"
	"github.com/galdor/go-log"
	"github.com/galdor/go-service/pkg/influx"
	"github.com/galdor/go-service/pkg/utils"
	"github.com/galdor/go-uuid"
)

var assetCacheBustingRE = regexp.MustCompile(
	`^(.+)\.(?:[a-z0-9]+)\.(css|js|jpg|png|woff2)$`)

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

func (h *Handler) UUIDQueryParameter(name string) (uuid.UUID, error) {
	s := h.QueryParameter(name)
	if s == "" {
		err := fmt.Errorf("missing or empty query parameter %q", name)
		h.ReplyError(400, "invalid_query_parameter", "%v", err)
		return uuid.Nil, err
	}

	var id uuid.UUID
	if err := id.Parse(s); err != nil {
		err := fmt.Errorf("invalid query parameter %q: %w", name, err)
		h.ReplyError(400, "invalid_query_parameter", "%v", err)
		return uuid.Nil, err
	}

	return id, nil
}

func (h *Handler) RequestData() ([]byte, error) {
	data, err := ioutil.ReadAll(h.Request.Body)
	if err != nil {
		h.ReplyInternalError(500, "cannot read request body: %v", err)
		return nil, fmt.Errorf("cannot read request body: %w", err)
	}

	return data, nil
}

func (h *Handler) JSONRequestData(dest interface{}) error {
	return h.JSONRequestData2(dest, nil)
}

func (h *Handler) JSONRequestData2(dest interface{}, fn func(*ejson.Validator)) error {
	data, err := h.RequestData()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		h.ReplyError(400, "invalid_request_body",
			"invalid request body: %v", err)
		return fmt.Errorf("invalid request body: %w", err)
	}

	if obj, ok := dest.(ejson.Validatable); ok {
		v := ejson.NewValidator()

		obj.ValidateJSON(v)

		if fn != nil {
			fn(v)
		}

		if err := v.Error(); err != nil {
			h.ReplyValidationErrors(err.(ejson.ValidationErrors))
			return err
		}
	}

	return nil
}

func (h *Handler) SetCookie(cookie *http.Cookie) {
	header := h.ResponseWriter.Header()
	header.Set("Set-Cookie", cookie.String())
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

func (h *Handler) ReplyJSON(status int, value interface{}) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")

	h.replyJSON(status, value, encoder, &buf)
}

func (h *Handler) ReplyCompactJSON(status int, value interface{}) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	h.replyJSON(status, value, encoder, &buf)
}

func (h *Handler) replyJSON(status int, value interface{}, encoder *json.Encoder, buf *bytes.Buffer) {
	header := h.ResponseWriter.Header()
	header.Set("Content-Type", "application/json")

	if err := encoder.Encode(value); err != nil {
		h.Log.Error("cannot encode json response: %v", err)
		h.ResponseWriter.WriteHeader(500)
		return
	}

	h.Reply(status, buf)
}

func (h *Handler) ReplyError(status int, code, format string, args ...interface{}) {
	h.ReplyErrorData(status, code, nil, format, args...)
}

func (h *Handler) ReplyErrorData(status int, code string, data ErrorData, format string, args ...interface{}) {
	h.errorCode = code
	h.Server.errorHandler(h, status, code, fmt.Sprintf(format, args...), data)
}

func (h *Handler) ReplyValidationErrors(err ejson.ValidationErrors) {
	data := ValidationJSONErrorData{
		ValidationErrors: err,
	}

	h.ReplyErrorData(400, "invalid_request_body", data,
		"invalid request body:\n%v", err)
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
		"event": "http.incoming_request",
		"time":  reqTime.Microseconds(),
	}

	if h.ClientAddress != "" {
		data["address"] = h.ClientAddress
	}

	if h.RequestId != "" {
		data["request_id"] = h.RequestId
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
		"req_time": reqTime.Microseconds(),
		"status":   w.Status,
	}

	if w.Status != 0 {
		fields["status"] = w.Status
	}

	point := influx.NewPointWithTimestamp("incoming_http_requests",
		tags, fields, now)

	h.Server.Cfg.InfluxClient.EnqueuePoint(point)
}
