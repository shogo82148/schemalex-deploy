package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/diff"
	"github.com/shogo82148/schemalex-deploy/internal/errors"
)

func main() {
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	var txn bool
	var version bool
	var outfile string

	flag.Usage = func() {
		// TODO: fill the usage
		fmt.Printf(`schemalex-deploy version %s

schemalex -version
`, getVersion())
	}
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

	if flag.NArg() != 2 {
		flag.Usage()
		return errors.New("wrong number of arguments")
	}

	var dst io.Writer = os.Stdout
	if len(outfile) > 0 {
		f, err := os.OpenFile(outfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return errors.Wrapf(err, `failed to open file %s for writing`, outfile)
		}
		dst = f
		defer f.Close()
	}

	fromSource, err := schemalex.NewSchemaSource(flag.Arg(0))
	if err != nil {
		return errors.Wrap(err, `failed to create schema source for "from"`)
	}

	toSource, err := schemalex.NewSchemaSource(flag.Arg(1))
	if err != nil {
		return errors.Wrap(err, `failed to create schema source for "to"`)
	}

	p := schemalex.New()
	return diff.Sources(
		dst,
		fromSource,
		toSource,
		diff.WithTransaction(txn), diff.WithParser(p),
	)
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
