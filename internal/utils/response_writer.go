package utils

import (
	"net/http"
)

type ResponseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

var _ http.ResponseWriter = (*ResponseWriterWrapper)(nil)

func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *ResponseWriterWrapper) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseWriterWrapper) Write(b []byte) (int, error) {
	return r.ResponseWriter.Write(b)
}

func (r *ResponseWriterWrapper) Header() http.Header {
	return r.ResponseWriter.Header()
}

func (r *ResponseWriterWrapper) GetStatusCode() int {
	return r.statusCode
}
