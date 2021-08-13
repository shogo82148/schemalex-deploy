package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"

	"github.com/shogo82148/schemalex-deploy/mycnf"
)

type config struct {
	version  bool
	host     string
	user     string
	password string
	database string
	port     int
	schema   []byte
}

func loadConfig() (*config, error) {
	var cfn config
	var version bool
	var host, username, password, database string
	var port int

	flag.Usage = func() {
		// TODO: fill the usage
		fmt.Printf(`schemalex-deploy version %s

schemalex -version
`, getVersion())
	}
	flag.StringVar(&host, "host", "", "the host name of the database")
	flag.IntVar(&port, "port", 3306, "the port number")
	flag.StringVar(&username, "user", "", "username")
	flag.StringVar(&password, "password", "", "password")
	flag.StringVar(&database, "database", "", "the database name")
	flag.BoolVar(&version, "version", false, "show the version")
	flag.Parse()

	if version {
		cfn.version = true
		return &cfn, nil
	}

	// load configure from files
	cnfFile, err := mycnf.LoadDefault("")
	if err != nil {
		return nil, err
	}
	if client, ok := cnfFile["client"]; ok {
		if v, ok := client["host"]; ok {
			cfn.host = v
		}
		if v, ok := client["port"]; ok {
			cfn.database = v
		}
		if v, ok := client["user"]; ok {
			cfn.user = v
		}
		if v, ok := client["password"]; ok {
			cfn.password = v
		}
		if v, ok := client["database"]; ok {
			cfn.database = v
		}
	}

	// load configure from the environment values
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfn.host = v
	}
	if v := os.Getenv("MYSQL_PWD"); v != "" {
		cfn.password = v
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("USER"); v != "" {
			cfn.user = v
		}
	} else {
		if cfn.user == "" {
			if u, err := user.Current(); err != nil {
				cfn.user = u.Username
			}
		}
	}
	if v := os.Getenv("MYSQL_TCP_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err != nil {
			cfn.port = i
		}
	}

	if host != "" {
		cfn.host = host
	}
	if port != 0 {
		cfn.port = port
	}
	if username != "" {
		cfn.user = username
	}
	if password != "" {
		cfn.password = password
	}
	if database != "" {
		cfn.database = database
	}

	if flag.NArg() == 0 {
		flag.Usage()
		return nil, errors.New("schema file is required")
	}
	schema, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		return nil, err
	}
	cfn.schema = schema

	return &cfn, nil
}
