package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func Run(args []string) error {
	app := pocketbase.New()

	var hooksDir string
	app.RootCmd.PersistentFlags().StringVar(
		&hooksDir,
		"hooksDir",
		"",
		"the directory with the JS app hooks",
	)

	app.RootCmd.ParseFlags(args)

	// load jsvm (pb_hooks and pb_migrations)
	jsvm.MustRegister(app, jsvm.Config{
		HooksDir: hooksDir,
	})

	// GitHub selfupdate
	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{
		Owner:             "btwiuse",
		Repo:              "pocketbase",
		ArchiveExecutable: "pocket",
	})

	// registers the relay middleware
	app.OnServe().Bind(RelayHook)

	return app.Start()
}
