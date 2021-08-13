package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-sql-driver/mysql"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/deploy"
	"golang.org/x/term"
)

func main() {
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	cfn, err := loadConfig()
	if err != nil {
		return err
	}
	if cfn.version {
		showVersion()
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	config := mysql.NewConfig()
	config.Addr = net.JoinHostPort(cfn.host, strconv.Itoa(cfn.port))
	config.User = cfn.user
	config.Passwd = cfn.password
	config.DBName = cfn.database
	config.ParseTime = true
	config.RejectReadOnly = true
	config.Params = map[string]string{
		"charset": "utf8mb4",
		// kamipo TRADITIONAL http://www.songmu.jp/riji/entry/2015-07-08-kamipo-traditional.html
		"sql_mode": "'TRADITIONAL,NO_AUTO_VALUE_ON_ZERO,ONLY_FULL_GROUP_BY'",
	}

	db, err := deploy.Open("mysql", config.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	// plan
	plan, err := db.Plan(ctx, string(cfn.schema))
	if err != nil {
		return fmt.Errorf("failed to plan: %w", err)
	}

	if !cfn.autoApprove {
		if result, err := approved(); err != nil {
			return err
		} else if !result {
			return errors.New("the plan was cancelled")
		}
	}

	// deploy
	if err := db.Deploy(ctx, plan); err != nil {
		return fmt.Errorf("failed to deploy: %w", err)
	}
	return nil
}

func approved() (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("tty is required")
	}
	fmt.Println("Do you want to perform these actions?")
	fmt.Println("Only 'yes' will be accepted to confirm.")
	fmt.Print("Enter a value: ")
	buf := bufio.NewReader(os.Stdin)
	line, err := buf.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(line)
	return line == "yes", nil
}

func showVersion() {
	fmt.Printf(
		"schemalex-deploy version %s, built with %s for %s/%s\n",
		getVersion(),
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
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
