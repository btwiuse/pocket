package main

import (
	"log"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/webteleport/relay"
)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func Run(args []string) error {
	app := pocketbase.New()

	app.RootCmd.ParseFlags(args)

	// GitHub selfupdate
	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{
		Owner:             "btwiuse",
		Repo:              "pocketbase",
		ArchiveExecutable: "pocket",
	})

	// registers the relay middleware
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(se *core.ServeEvent) error {
			log.Println("starting the relay server", "HOST", apis.HOST)

			store := relay.NewSessionStore()
			mini := relay.NewWSServer(apis.HOST, store)

			se.Router.BindFunc(func(re *core.RequestEvent) error {
				isPocketbaseHost := mini.IsRootExternal(re.Event.Request)
				isPocketbaseAPI := strings.HasPrefix(re.Event.Request.URL.Path, "/api/")
				isPocketbaseUI := strings.HasPrefix(re.Event.Request.URL.Path, "/_/")
				isPocketbase := isPocketbaseHost && (isPocketbaseAPI || isPocketbaseUI)

				// route non pocketbase requests to relay
				if !isPocketbase {
					mini.ServeHTTP(re.Event.Response, re.Event.Request)
					return nil
				}

				return re.Next()
			})

			return se.Next()
		},
		Priority: -99999, // execute as early as possible
	})

	return app.Start()
}
