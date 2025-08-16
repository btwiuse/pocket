package router

import "fmt"

func (e *Event) GetTrace() string {
	var old = e.Get(TraceMiddlewareKey)
	if old == nil {
		return "::trace::"
	}
	return fmt.Sprintf("%v", old)
}

var TraceMiddlewareKey = "mw_trace"

func (e *Event) TraceMiddleware(id string, pri int) {
	e.Set(TraceMiddlewareKey, fmt.Sprintf("%s => %s (%d)", e.GetTrace(), id, pri))
}
