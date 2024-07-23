package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/shogo82148/schemalex-deploy/deploy"
	"golang.org/x/term"
)

// the version is set by goreleaser
var version = "" // .Version

func main() {
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	cfn, err := loadConfig(os.Args)
	if err != nil {
		return err
	}
	if cfn.Version {
		fmt.Println(getVersion())
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	config := mysql.NewConfig()
	if cfn.Socket != "" {
		config.Net = "unix"
		config.Addr = cfn.Socket
	} else {
		config.Net = "tcp"
		config.Addr = net.JoinHostPort(cfn.Host, strconv.Itoa(cfn.Port))
	}
	config.User = cfn.User
	config.Passwd = cfn.Password
	config.DBName = cfn.Database
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

	switch cfn.Mode {
	case ExecModeDeploy:
		return runDeploy(ctx, db, cfn)

	case ExecModeImport:
		return runImport(ctx, db, cfn)
	}

	return nil
}

func runDeploy(ctx context.Context, db *deploy.DB, cfn *config) error {
	// plan
	plan, err := db.Plan(ctx, string(cfn.Schema))
	if err != nil {
		return fmt.Errorf("failed to plan: %w", err)
	}

	// preview
	if err := plan.Preview(os.Stderr); err != nil {
		return fmt.Errorf("failed to preview: %w", err)
	}

	// dry-run mode: skip deployment
	if cfn.DryRun {
		return nil
	}

	// ask to approve
	if !cfn.AutoApprove {
		if result, err := approved(ctx); err != nil {
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

func runImport(ctx context.Context, db *deploy.DB, cfn *config) error {
	// load schema
	sqlText, err := db.LoadSchema(ctx)
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	// preview
	if _, err := io.WriteString(os.Stderr, sqlText); err != nil {
		return fmt.Errorf("failed to preview: %w", err)
	}

	// no table detected, skip import
	if sqlText == "" {
		log.Print("no table detected")
		return nil
	}

	// dry-run mode: skip import
	if cfn.DryRun {
		return nil
	}

	// ask to approve
	if !cfn.AutoApprove {
		if result, err := approved(ctx); err != nil {
			return err
		} else if !result {
			return errors.New("import was cancelled")
		}
	}

	if err := db.Import(ctx, sqlText); err != nil {
		return fmt.Errorf("failed to import: %w", err)
	}

	return nil
}

func approved(ctx context.Context) (bool, error) {
	type result struct {
		line string
		err  error
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("tty is required")
	}
	fmt.Println("Do you want to perform these actions?")
	fmt.Println("Only 'yes' will be accepted to confirm.")
	fmt.Print("Enter a value: ")

	ch := make(chan result, 1)
	go func() {
		buf := bufio.NewReader(os.Stdin)
		line, err := buf.ReadString('\n')
		if err != nil {
			ch <- result{err: err}
			return
		}
		ch <- result{line: strings.TrimSpace(line)}
	}()

	select {
	case ret := <-ch:
		return ret.line == "yes", ret.err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func getVersion() string {
	var revision string
	var time string
	var modified bool
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "" {
			version = info.Main.Version
		}
		for _, kv := range info.Settings {
			switch kv.Key {
			case "vcs.revision":
				revision = kv.Value
			case "vcs.time":
				time = kv.Value
			case "vcs.modified":
				if b, err := strconv.ParseBool(kv.Value); err == nil {
					modified = b
				}
			}
		}
	}

	var buf strings.Builder
	buf.WriteString("schemalex-deploy version ")
	if version != "" {
		buf.WriteString(version)
	} else {
		buf.WriteString("unknown")
	}
	if revision != "" {
		buf.WriteString(" (")
		buf.WriteString(revision)
		buf.WriteString(" at ")
		buf.WriteString(time)
		if modified {
			buf.WriteString(", modified")
		}
		buf.WriteString(")")
	}
	buf.WriteString(", built with ")
	buf.WriteString(runtime.Version())
	buf.WriteString(" for ")
	buf.WriteString(runtime.GOOS)
	buf.WriteString("/")
	buf.WriteString(runtime.GOARCH)
	return buf.String()
}
