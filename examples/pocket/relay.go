package main

import (
	"log"
	"os"
	"strconv"

	"github.com/btwiuse/proxy"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/webteleport/relay"
	"github.com/webteleport/utils"
)

// disable the relay server by setting HOST to an empty string
var HOST = utils.EnvHost("")

// execute as late as possible to make user provided hooks run early
var PRIORITY = StringToInt(os.Getenv("PRIORITY"), 99998)

var RelayHook = &hook.Handler[*core.ServeEvent]{
	Id:       "RelayHookId",
	Func:     RelayHookFunc,
	Priority: PRIORITY,
}

func StringToInt(str string, fallback int) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		return fallback
	}
	return num
}

func RelayHookFunc(se *core.ServeEvent) error {
	if HOST == "" {
		return se.Next()
	}

	log.Println("starting the relay server", "HOST", HOST)

	s := relay.DefaultWSServer(HOST)

	proxyMiddleware := &hook.Handler[*core.RequestEvent]{
		Id: "proxyMiddlewareId",
		Func: func(re *core.RequestEvent) error {
			w, r := re.Event.Response, re.Event.Request

			if proxy.IsProxy(r) {
				proxy.AuthenticatedProxyHandler.ServeHTTP(w, r)
				return nil
			}

			return re.Next()
		},
		Priority: 1,
	}

	upgradeMiddleware := &hook.Handler[*core.RequestEvent]{
		Id: "upgradeMiddlewareId",
		Func: func(re *core.RequestEvent) error {
			w, r := re.Event.Response, re.Event.Request

			if s.IsUpgrade(r) {
				s.HTTPUpgrader.ServeHTTP(w, r)
				return nil
			}

			return re.Next()
		},
		Priority: 2,
	}

	ingressMiddleware := &hook.Handler[*core.RequestEvent]{
		Id: "ingressMiddlewareId",
		Func: func(re *core.RequestEvent) error {
			w, r := re.Event.Response, re.Event.Request

			if !s.IsRootExternal(r) {
				s.Ingress.ServeHTTP(w, r)
				return nil
			}

			return re.Next()
		},
		Priority: 3,
	}

	se.Router.Bind(proxyMiddleware)

	se.Router.Bind(upgradeMiddleware)

	se.Router.Bind(ingressMiddleware)

	return se.Next()
}
