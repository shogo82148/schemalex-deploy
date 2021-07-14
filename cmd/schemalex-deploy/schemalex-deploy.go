package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"runtime/debug"

	"github.com/shogo82148/schemalex-deploy"
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

	fmt.Println("TODO: implement")
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
