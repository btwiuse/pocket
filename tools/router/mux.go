package router

import (
	"net/http"
)

type ServeMux struct {
	*http.ServeMux
	connectHandler http.Handler
}

func (m *ServeMux) HandleFunc(pat string, h http.HandlerFunc) {
	m.Handle(pat, http.HandlerFunc(h))
}

func (m *ServeMux) Handle(pat string, h http.Handler) {
	if pat == "CONNECT " {
		m.connectHandler = h
		return
	}
	m.ServeMux.Handle(pat, h)
}

func (m *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect && r.URL.Path == "" && m.connectHandler != nil {
		m.connectHandler.ServeHTTP(w, r)
		return
	}
	m.ServeMux.ServeHTTP(w, r)
}

func (m *ServeMux) Handler(r *http.Request) (h http.Handler, pat string) {
	if r.Method == http.MethodConnect && r.URL.Path == "" && m.connectHandler != nil {
		return m.connectHandler, ""
	}
	return m.ServeMux.Handler(r)
}

func NewServeMux() *ServeMux {
	m := http.NewServeMux()
	return &ServeMux{
		ServeMux: m,
	}
}
