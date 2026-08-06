package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"

	"github.com/shogo82148/schemalex-deploy/mycnf"
)

// ExecMode execute mode
type ExecMode string

const (
	// ExecModeDeploy deploy mode
	ExecModeDeploy ExecMode = "deploy"
	// ExecModeImport import mode
	ExecModeImport ExecMode = "import"
)

type config struct {
	Version     bool
	Socket      string
	Host        string
	User        string
	Password    string
	Database    string
	Port        int
	SSLMode     SSLMode
	SSLCA       string
	Schema      []byte
	AutoApprove bool
	DryRun      bool
	Mode        ExecMode
}

// for testing
var loadDefault = mycnf.LoadDefault

// lookupOption looks up the option named name in the group.
// mysql(1) treats dashes and underscores in option names interchangeably.
// https://dev.mysql.com/doc/refman/8.0/en/option-files.html
func lookupOption(group map[string]string, name string) (string, bool) {
	if v, ok := group[name]; ok {
		return v, true
	}
	v, ok := group[strings.ReplaceAll(name, "-", "_")]
	return v, ok
}

func loadConfig(args []string) (*config, error) {
	var cfn config
	var version bool
	var socket string
	var host, username, password, database string
	var port int
	var sslMode, sslCA string
	var approve bool
	var dryRun bool
	var runImport bool

	flagSet := flag.NewFlagSet(args[0], flag.ExitOnError)

	flagSet.Usage = func() {
		fmt.Printf(`schemalex-deploy version %s

-socket           the unix domain socket path for the database
-host             the host name of the database
-port             the port number(default: 3306)
-user             username
-password         password
-database         the database name
-ssl-mode         the security state of the connection to the database
                  (DISABLED, PREFERRED, REQUIRED, VERIFY_CA or VERIFY_IDENTITY.
                  default: PREFERRED, or VERIFY_CA if -ssl-ca is specified)
-ssl-ca           the path of the file that contains the CA certificate
                  (used by VERIFY_CA and VERIFY_IDENTITY)
-version          show the version
-auto-approve     skips interactive approval of plan before deploying
-dry-run          outputs the schema difference, and then exit the program
-import           imports existing table schemas from running database
`, getVersion())
	}

	// options that are compatible with the mysql(1)
	// https://dev.mysql.com/doc/refman/8.0/en/mysql-command-options.html
	flagSet.StringVar(&socket, "socket", "", "the unix domain socket path for the database")
	flagSet.StringVar(&host, "host", "", "the host name of the database")
	flagSet.IntVar(&port, "port", 0, "the port number")
	flagSet.StringVar(&username, "user", "", "username")
	flagSet.StringVar(&password, "password", "", "password")
	flagSet.StringVar(&database, "database", "", "the database name")
	flagSet.StringVar(&sslMode, "ssl-mode", "", "the security state of the connection to the database")
	flagSet.StringVar(&sslCA, "ssl-ca", "", "the path of the file that contains the CA certificate")
	flagSet.BoolVar(&version, "version", false, "show the version")

	// for schemalex-deploy
	flagSet.BoolVar(&approve, "auto-approve", false, "skips interactive approval of plan before deploying")
	flagSet.BoolVar(&dryRun, "dry-run", false, "outputs the schema difference, and then exit the program")
	flagSet.BoolVar(&runImport, "import", false, "imports existing table schemas from running database")
	if err := flagSet.Parse(args[1:]); err != nil {
		return nil, err
	}

	if version {
		cfn.Version = true
		return &cfn, nil
	}

	cfn.AutoApprove = approve
	cfn.DryRun = dryRun
	cfn.Port = 3306

	// choose execute mode
	cfn.Mode = ExecModeDeploy
	if runImport {
		cfn.Mode = ExecModeImport
	}

	// load configure from files
	cnfFile, err := loadDefault("")
	if err != nil {
		return nil, err
	}
	if client, ok := cnfFile["client"]; ok {
		if v, ok := client["socket"]; ok {
			cfn.Socket = v
		}
		if v, ok := client["host"]; ok {
			cfn.Host = v
		}
		if v, ok := client["port"]; ok {
			if i, err := strconv.Atoi(v); err == nil { // if NO error
				cfn.Port = i
			}
		}
		if v, ok := client["user"]; ok {
			cfn.User = v
		}
		if v, ok := client["password"]; ok {
			cfn.Password = v
		}
		if v, ok := client["database"]; ok {
			cfn.Database = v
		}
		if v, ok := lookupOption(client, "ssl-mode"); ok {
			mode, err := parseSSLMode(v)
			if err != nil {
				return nil, err
			}
			cfn.SSLMode = mode
		}
		if v, ok := lookupOption(client, "ssl-ca"); ok {
			cfn.SSLCA = v
		}
	}

	// load configure from the environment values
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	if v := os.Getenv("MYSQL_UNIX_PORT"); v != "" {
		cfn.Socket = v
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfn.Host = v
	}
	if v := os.Getenv("MYSQL_PWD"); v != "" {
		cfn.Password = v
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("USER"); v != "" {
			cfn.User = v
		}
	} else {
		if cfn.User == "" {
			if u, err := user.Current(); err == nil { // if NO error
				cfn.User = u.Username
			}
		}
	}
	if v := os.Getenv("MYSQL_TCP_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil { // if NO error
			cfn.Port = i
		}
	}

	if socket != "" {
		cfn.Socket = socket
	}
	if host != "" {
		cfn.Host = host
	}
	if port != 0 {
		cfn.Port = port
	}
	if username != "" {
		cfn.User = username
	}
	if password != "" {
		cfn.Password = password
	}
	if database != "" {
		cfn.Database = database
	}
	if sslMode != "" {
		mode, err := parseSSLMode(sslMode)
		if err != nil {
			return nil, err
		}
		cfn.SSLMode = mode
	}
	if sslCA != "" {
		cfn.SSLCA = sslCA
	}

	// if the ssl-mode is not explicitly set, use of the ssl-ca implies VERIFY_CA.
	// https://dev.mysql.com/doc/refman/8.0/en/connection-options.html#option_general_ssl-ca
	if cfn.SSLMode == "" {
		if cfn.SSLCA != "" {
			cfn.SSLMode = SSLModeVerifyCA
		} else {
			cfn.SSLMode = SSLModePreferred
		}
	}

	// deploy mode: load schema file
	if cfn.Mode == ExecModeDeploy {
		if flagSet.NArg() == 0 {
			flagSet.Usage()
			return nil, errors.New("schema file is required")
		}
		schema, err := os.ReadFile(flagSet.Arg(0))
		if err != nil {
			return nil, err
		}
		cfn.Schema = schema
	}

	return &cfn, nil
}
