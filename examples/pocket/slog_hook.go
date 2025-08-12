package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/btwiuse/better"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

var SlogHookPriority = apis.DefaultActivityLoggerMiddlewarePriority - 1

var SlogHook = &hook.Handler[*core.ServeEvent]{
	Id:       "SlogHookId",
	Func:     SlogHookFunc,
	Priority: SlogHookPriority,
}

func SlogHookFunc(se *core.ServeEvent) error {
	log.Println("Binding the slog hook.", "Priority:", SlogHookPriority)

	var slogPrefix = "/slog"
	var slogHandler http.Handler = http.StripPrefix(
		slogPrefix,
		better.FileServer(http.Dir(".")),
	)

	se.Router.BindFunc(func(re *core.RequestEvent) error {
		w, r := re.Event.Response, re.Event.Request
		logger := re.App.Logger().
			With("app", "pocket").
			With("type", "slog").
			With("hook", "SlogHook")

		isSlog := strings.HasPrefix(r.URL.Path, slogPrefix)

		if isSlog {
			slogHandler.ServeHTTP(w, r)
			msg := fmt.Sprintf("SLOG %s", r.URL.RequestURI())
			attrs := make([]any, 0, 15)
			attrs = append(attrs,
				slog.String("path", r.URL.Path),
				slog.String("url", r.URL.RequestURI()),
				slog.String("host", r.Host),
				slog.String("method", r.Method),
				slog.String("referer", r.Referer()),
				slog.String("userAgent", r.UserAgent()),
				slog.Int("status", re.Status()),
				slog.String("userIP", re.RealIP()),
				slog.String("remoteIP", re.RemoteIP()),
			)
			logger.Info(msg, attrs...)
			return nil
		}

		return re.Next()
	})

	return se.Next()
}
