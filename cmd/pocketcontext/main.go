package main

import (
	"errors"
	"log"
	"os"

	"github.com/amiorin/pocketcontext/internal/database"
	"github.com/amiorin/pocketcontext/internal/server"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/pflag"
)

func main() {
	app := pocketbase.NewWithConfig(pocketbase.Config{DBConnect: database.Connect})
	var configPath, hooksDir, migrationsDir string
	app.RootCmd.PersistentFlags().StringVar(&configPath, "contextConfig", "pocketcontext.json", "SQL read configuration file")
	app.RootCmd.PersistentFlags().StringVar(&hooksDir, "hooksDir", "pb_hooks", "application JavaScript hooks directory")
	app.RootCmd.PersistentFlags().StringVar(&migrationsDir, "migrationsDir", "pb_migrations", "application JavaScript migrations directory")
	if err := app.RootCmd.ParseFlags(os.Args[1:]); err != nil && !errors.Is(err, pflag.ErrHelp) {
		log.Fatal(err)
	}
	jsvm.MustRegister(app, jsvm.Config{HooksDir: hooksDir, MigrationsDir: migrationsDir, HooksWatch: false})
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{Dir: migrationsDir, TemplateLang: migratecmd.TemplateLangJS, Automigrate: false})
	server.Register(app, configPath)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
