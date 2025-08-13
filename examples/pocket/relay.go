package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

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
	// 3. set cookies, 302 redirected to HOST
	// 4. serves agent content
	// 5. user visits HOST/_/, unsetting cookies
	// 6. user visits fallback path

	// pros
	// - accessible only from authenticated sessions

	// cons:
	// - browser only
	// - expose one app at a time
	// - browser reauthenticate needed when agent reconnects

	indexSessionKey := "_indexSessionKey"
	unsetCookie := func(w http.ResponseWriter) {
		http.SetCookie(w, &http.Cookie{
			Name:     indexSessionKey,
			Value:    "",
			MaxAge:   -1,
			Path:     "/",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
	indexMiddleware := &hook.Handler[*core.RequestEvent]{
		Id: "indexMiddlewareId",
		Func: func(re *core.RequestEvent) error {
			w, r := re.Event.Response, re.Event.Request

			isUI := strings.HasPrefix(r.URL.Path, "/_/")
			isAPI := strings.HasPrefix(r.URL.Path, "/api/")
			isUIReferer := strings.HasSuffix(r.Referer(), "/_/")

			if s.IsRootExternal(r) && isAPI && isUIReferer {
				// passthrough requests to /api/... from /_/
				goto NEXT
			}

			if s.IsRootExternal(r) && isUI {
				// reset session cookie on visiting /_/
				unsetCookie(w)
				goto NEXT
			}

			if s.IsRootExternal(r) && !isUI {
				// get path => id
				rpath := leadingComponent(r.URL.Path)

				// set session cookie
				if rt, ok := s.GetRoundTripper(rpath); ok {
					// enable reverse proxy
					rp := utils.LoggedReverseProxy(rt)
					rp.Rewrite = func(req *httputil.ProxyRequest) {
						req.SetXForwarded()
						req.Out.URL.Host = r.Host
						req.Out.URL.Scheme = "http"
					}
					re.App.Store().Set(rpath, rp)
					// set cookies and redirect to /
					http.SetCookie(w, &http.Cookie{
						Name:     indexSessionKey,
						Value:    rpath,
						MaxAge:   2592000, // 30 days in seconds
						Path:     "/",
						HttpOnly: true,
						SameSite: http.SameSiteLaxMode,
					})
					http.Redirect(w, r, "/", http.StatusFound)
					return nil
				} else {
					// check cookie
					cookie, err := r.Cookie(indexSessionKey)
					if err != nil {
						goto NEXT
					}
					if _, ok := relay.DefaultStorage.LookupRecord(cookie.Value); ok {
						// serve subsequent calls
						rp, ok := re.App.Store().GetOk(cookie.Value)
						if ok {
							rp.(http.Handler).ServeHTTP(w, r)
							return nil
						}
					} else {
						// invalidate cookie
						unsetCookie(w)
					}
				}
			}

		NEXT:
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
