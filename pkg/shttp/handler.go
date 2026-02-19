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

	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/program"
	"go.n16f.net/service/pkg/influx"
	"go.n16f.net/service/pkg/utils"
	"go.n16f.net/uuid"
)

var assetCacheBustingRE = regexp.MustCompile(
	`^(.+)\.(?:[a-z0-9]+)\.(css|js|jpg|png|woff2)$`)

type Handler struct {
	Server *Server
	Log    *log.Logger

	Method      string
	PathPattern string
	RouteId     string // based on the method and path pattern
	Options     RouteOptions

	Request        *http.Request
	Query          url.Values
	ResponseWriter http.ResponseWriter

	ClientAddress string // optional
	RequestId     string // optional

	start     time.Time
	errorCode string
}

func (h *Handler) PathVariable(name string) string {
	value := h.Request.PathValue(name)
	if value == "" {
		program.Panic("unknown path variable %q", name)
	}

	return value
}

func (h *Handler) UUIDPathVariable(name string) (uuid.UUID, error) {
	s := h.PathVariable(name)

	var id uuid.UUID
	if err := id.Parse(s); err != nil {
		err := fmt.Errorf("invalid path segment %q: %w", s, err)
		h.ReplyError(400, "invalid_path_segment", "%v", err)
		return uuid.Nil, err
	}

	return id, nil
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

func (h *Handler) JSONRequestData(dest any) error {
	data, err := h.RequestData()
	if err != nil {
		return err
	}

	return h.JSONRequestDataExt(data, dest, nil)
}

// The extended version of JSONRequestData is useful in specific situations:
//
// 1. JSON data must be extracted and/or transformed from the request body
// before decoding.
//
// 2. Extra validation steps must be performed with the same Validator object.
func (h *Handler) JSONRequestDataExt(data []byte, dest any, fn func(*ejson.Validator) error) error {
	if err := json.Unmarshal(data, dest); err != nil {
		h.ReplyError(400, "invalid_request_body",
			"invalid request body: %v", err)
		return fmt.Errorf("invalid request body: %w", err)
	}

	if obj, ok := dest.(ejson.Validatable); ok {
		v := ejson.NewValidator()

		obj.ValidateJSON(v)

		if fn != nil {
			if err := fn(v); err != nil {
				return err
			}
		}

		if err := v.Error(); err != nil {
			h.ReplyValidationErrors(err.(ejson.ValidationErrors))
			return err
		}
	}

	return nil
}

func (h *Handler) AcceptedMediaRanges() MediaRanges {
	accept := h.Request.Header.Get("Accept")
	if accept == "" {
		return nil
	}

	var mrs MediaRanges
	mrs.Parse(accept)

	return mrs
}

func (h *Handler) AddCookie(cookie *http.Cookie) {
	header := h.ResponseWriter.Header()
	header.Add("Set-Cookie", cookie.String())
}

func (h *Handler) SetCookie(cookie *http.Cookie) {
	header := h.ResponseWriter.Header()
	header.Set("Set-Cookie", cookie.String())
}

func (h *Handler) SSELastEventId() string {
	return h.Request.Header.Get("Last-Event-ID")
}

func (h *Handler) SSEInt64LastEventId() (int64, error) {
	s := h.SSELastEventId()
	if s == "" {
		return 0, nil
	}

	i64, err := strconv.ParseInt(s, 10, 64)
	if err != nil || i64 < 1 {
		err = errors.New("invalid SSE last event id")
		h.ReplyError(400, "invalid_sse_last_event_id", "%v", err)
		return 0, err
	}

	return i64, nil
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

func (h *Handler) ReplyJSON(status int, value any) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")

	h.replyJSON(status, value, encoder, &buf)
}

func (h *Handler) ReplyCompactJSON(status int, value any) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	h.replyJSON(status, value, encoder, &buf)
}

func (h *Handler) replyJSON(status int, value any, encoder *json.Encoder, buf *bytes.Buffer) {
	header := h.ResponseWriter.Header()
	header.Set("Content-Type", "application/json")

	if err := encoder.Encode(value); err != nil {
		h.Log.Error("cannot encode JSON response: %v", err)
		h.ResponseWriter.WriteHeader(500)
		return
	}

	h.Reply(status, buf)
}

func (h *Handler) ReplyError(status int, code, format string, args ...any) {
	h.ReplyErrorData(status, code, nil, format, args...)
}

func (h *Handler) ReplyErrorData(status int, code string, data ErrorData, format string, args ...any) {
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

func (h *Handler) ReplyInternalError(status int, format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
	h.Log.Error("internal error: %s", msg)

	if h.Server.Cfg.HideInternalErrors {
		msg = "internal error"
	}

	h.ReplyError(status, "internal_error", "%s", msg)
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
		h.ReplyError(400, "non_regular_file",
			"%q is not a regular file", filePath)
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

func (h *Handler) ReplyChunk(r io.Reader) error {
	if _, err := io.Copy(h.ResponseWriter, r); err != nil {
		err2 := fmt.Errorf("cannot copy response chunk: %v", err)
		h.Log.Error("%v", err2)
		return err2
	}

	h.ResponseWriter.(http.Flusher).Flush()
	return nil
}

func (h *Handler) ReplyJSONChunk(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		err2 := fmt.Errorf("cannot encode JSON response chunk: %v", err)
		h.Log.Error("%v", err2)
		return err2
	}

	data = append(data, '\n')

	return h.ReplyChunk(bytes.NewReader(data))
}

func (h *Handler) ReplySSE(status int) error {
	header := h.ResponseWriter.Header()
	header.Set("Content-Type", "text/event-stream")

	h.ResponseWriter.WriteHeader(status)
	return nil
}

func (h *Handler) WriteSSE(id, event, data string) error {
	var buf bytes.Buffer

	if id != "" {
		buf.WriteString("id: ")
		buf.WriteString(id)
		buf.WriteByte('\n')
	}

	if event != "" {
		buf.WriteString("event: ")
		buf.WriteString(event)
		buf.WriteByte('\n')
	}

	if data != "" {
		buf.WriteString("data: ")
		buf.WriteString(data)
		buf.WriteByte('\n')
	}

	buf.WriteByte('\n')

	if _, err := io.Copy(h.ResponseWriter, &buf); err != nil {
		return err
	}

	h.ResponseWriter.(http.Flusher).Flush()
	return nil
}

func (h *Handler) WriteSSEComment(comment string) error {
	line := ": " + comment + "\n\n"
	if _, err := h.ResponseWriter.Write([]byte(line)); err != nil {
		return err
	}

	h.ResponseWriter.(http.Flusher).Flush()
	return nil
}

func (h *Handler) WriteJSONSSE(id, event string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		err2 := fmt.Errorf("cannot encode JSON event data: %v", err)
		h.Log.Error("%v", err2)
		return err2
	}

	return h.WriteSSE(id, event, string(data))
}

func (h *Handler) WriteSSEError(code, format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\n")

	err := JSONError{
		Code:    code,
		Message: msg,
	}

	h.WriteJSONSSE("", "error", &err)
}

func (h *Handler) WriteSSEInternalError(format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
	h.Log.Error("internal error: %s", msg)

	if h.Server.Cfg.HideInternalErrors {
		msg = "internal error"
	}

	err := JSONError{
		Code:    "internal_error",
		Message: msg,
	}

	h.WriteJSONSSE("", "error", &err)
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
	if h.Options.DisableAccessLog {
		return
	}

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

	var status string
	if w.Status >= 200 && w.Status < 300 {
		status = "2xx"
	} else if w.Status >= 300 && w.Status < 400 {
		status = "3xx"
	} else if w.Status >= 400 && w.Status < 500 {
		status = "4xx"
	} else if w.Status >= 500 && w.Status < 600 {
		status = "5xx"
	}
	if status != "" {
		tags["status"] = status
	}

	fields := influx.Fields{
		"req_time":    reqTime.Microseconds(),
		"status_code": w.Status,
	}

	if w.Status != 0 {
		fields["status_code"] = w.Status
	}

	point := influx.NewPointWithTimestamp("incoming_http_requests",
		tags, fields, now)

	h.Server.Cfg.InfluxClient.EnqueuePoint(point)
}
