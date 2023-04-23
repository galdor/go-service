package shttp

import "net/http"

type ResponseWriter struct {
	Status int

	w http.ResponseWriter
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		w: w,
	}
}

func (w *ResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *ResponseWriter) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.Status = status

	w.w.WriteHeader(status)
}
