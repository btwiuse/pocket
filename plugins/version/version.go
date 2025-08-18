// Package version implements a new command to print version
//
// Example usage:
//
//	version.MustRegister(app, app.RootCmd, version.Config{})
package version

import (
	"encoding/json"
	"fmt"

	ver "github.com/btwiuse/version"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Config defines the config options of the ghupdate plugin.
//
// NB! This plugin is considered experimental and its config options may change in the future.
type Config struct{}

// MustRegister registers the ghupdate plugin to the provided app instance
// and panic if it fails.
func MustRegister(app core.App, rootCmd *cobra.Command, config Config) {
	if err := Register(app, rootCmd, config); err != nil {
		panic(err)
	}
}

// Register registers the ghupdate plugin to the provided app instance.
func Register(app core.App, rootCmd *cobra.Command, config Config) error {
	p := &plugin{
		app:            app,
		currentVersion: rootCmd.Version,
		config:         config,
	}

	rootCmd.AddCommand(p.versionCmd())

	return nil
}

type plugin struct {
	app            core.App
	config         Config
	currentVersion string
}

func (p *plugin) versionCmd() *cobra.Command {
	command := &cobra.Command{
		Use:          "version",
		Short:        "Print version",
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			return Run(args)
		},
	}

	return command
}

func Run(args []string) error {
	v := ver.Info
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
	return nil
}
