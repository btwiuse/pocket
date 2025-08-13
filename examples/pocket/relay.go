package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

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

func leadingComponent(s string) string {
	return strings.Split(strings.TrimPrefix(s, "/"), "/")[0]
}

func RelayHookFunc(se *core.ServeEvent) error {
	if HOST == "" {
		return se.Next()
	}

	log.Println("starting the relay server", "HOST", HOST)

	relay.DefaultStorage.OnUpdateFunc = func(t *relay.Store) {
		store := se.App.Store()
		store.Set("relayRecordMap", t.RecordMap)
		store.Set("relayAliasMap", t.AliasMap)
	}

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

	// request flow:
	// 1. agent register as <id>
	// 2. user visits HOST/<id>
	// 3. update indexHandler, 302 redirected to HOST
	// 4. serves agent content
	// 5. agent disconnects, resets indexHandler
	// 6. user visits fallback path

	// pros
	// - works for cli clients and browsers

	// cons:
	// - accessible by everyone once exposed

	var indexHandler http.Handler
	var indexPath string

	indexMiddleware := &hook.Handler[*core.RequestEvent]{
		Id: "indexMiddlewareId",
		Func: func(re *core.RequestEvent) error {
			w, r := re.Event.Response, re.Event.Request

			isAPI := strings.HasPrefix(r.URL.Path, "/api/")
			isUI := strings.HasPrefix(r.URL.Path, "/_/")

			if s.IsRootExternal(r) && !isAPI && !isUI {
				// get path => id
				rpath := leadingComponent(r.URL.Path)
				if rt, ok := s.GetRoundTripper(rpath); ok {
					// set subsequent calls
					rp := utils.LoggedReverseProxy(rt)
					rp.Rewrite = func(req *httputil.ProxyRequest) {
						req.SetXForwarded()
						req.Out.URL.Host = r.Host
						req.Out.URL.Scheme = "http"
					}
					indexHandler = rp
					indexPath = rpath
					// redirect to /
					http.Redirect(w, r, "/", http.StatusFound)
					return nil
				} else if indexHandler != nil {
					if _, ok := relay.DefaultStorage.LookupRecord(indexPath); ok {
						indexHandler.ServeHTTP(w, r)
						return nil
					}
					indexPath = ""
					indexHandler = nil
				}
			}

			return re.Next()
		},
		Priority: 3,
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
		Priority: 4,
	}

	se.Router.Bind(proxyMiddleware)

	se.Router.Bind(upgradeMiddleware)

	se.Router.Bind(indexMiddleware)

	se.Router.Bind(ingressMiddleware)

	return se.Next()
}
