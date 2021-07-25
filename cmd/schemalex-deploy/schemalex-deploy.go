package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/deploy"
)

func main() {
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	var version bool

	flag.Usage = func() {
		// TODO: fill the usage
		fmt.Printf(`schemalex-deploy version %s

schemalex -version
`, getVersion())
	}
	flag.BoolVar(&version, "version", false, "show the version")
	flag.Parse()

	if version {
		fmt.Printf(
			"schemalex-deploy version %s, built with go %s for %s/%s\n",
			getVersion(),
			runtime.Version(),
			runtime.GOOS,
			runtime.GOARCH,
		)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := deploy.Open("mysql", "TODO: fill me!")
	if err != nil {
		return err
	}
	defer db.Close()

	_ = ctx
	// TODO: implement
	// if err := db.Deploy(ctx); err != nil {
	// 	return err
	// }
	return nil
}

func getVersion() string {
	if schemalex.Version != "" {
		// in case of release build
		return schemalex.Version
	}

	// in case of built from source
	i, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return i.Main.Version
}
