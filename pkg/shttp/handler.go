package shttp

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/utils"
)

type Handler struct {
	Server *Server
	Log    *log.Logger

	Request        *http.Request
	ResponseWriter http.ResponseWriter

	Query url.Values

	pathVariables map[string]string
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
