package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy/mycnf"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		cnf  mycnf.MyCnf
		want *config

		// environment variables
		unixPort string
		tcpPort  string
		host     string
		pwd      string
	}{
		{
			name: "show version",
			args: []string{"schemalex-deploy", "-version"},
			cnf:  mycnf.MyCnf{},
			want: &config{
				Version: true,
			},
		},

		// password
		{
			name: "The password specified in the argument takes precedence",
			args: []string{"schemalex-deploy", "-user", "shogo", "-password", "secret", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"user":     "chooblarin",
					"password": "password",
				},
			},
			pwd: "environment",
			want: &config{
				User:     "shogo",
				Password: "secret",
				Port:     3306,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},
		{
			name: "The password specified in the environment values takes precedence",
			args: []string{"schemalex-deploy", "-user", "shogo", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"user":     "chooblarin",
					"password": "password",
				},
			},
			pwd: "environment",
			want: &config{
				User:     "shogo",
				Password: "environment",
				Port:     3306,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},
		{
			name: "The password specified in the configure file takes precedence",
			args: []string{"schemalex-deploy", "-user", "shogo", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"user":     "chooblarin",
					"password": "password",
				},
			},
			want: &config{
				User:     "shogo",
				Password: "password",
				Port:     3306,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},

		// port number
		{
			name: "The port-number specified in the argument takes precedence",
			args: []string{"schemalex-deploy", "-port", "1234", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"port":     "2345",
					"user":     "chooblarin",
					"password": "password",
				},
			},
			tcpPort: "3456",
			want: &config{
				User:     "chooblarin",
				Password: "password",
				Port:     1234,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},
		{
			name: "The port-number specified in the environment values takes precedence",
			args: []string{"schemalex-deploy", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"port":     "2345",
					"user":     "chooblarin",
					"password": "password",
				},
			},
			tcpPort: "3456",
			want: &config{
				User:     "chooblarin",
				Password: "password",
				Port:     3456,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},
		{
			name: "The port-number specified in the configure file values takes precedence",
			args: []string{"schemalex-deploy", filepath.Join("testdata", "schema.sql")},
			cnf: mycnf.MyCnf{
				"client": map[string]string{
					"port":     "2345",
					"user":     "chooblarin",
					"password": "password",
				},
			},
			want: &config{
				User:     "chooblarin",
				Password: "password",
				Port:     2345,
				Schema:   []byte{},
				Mode:     ExecModeDeploy,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := loadDefault
			loadDefault = func(extraFile string) (mycnf.MyCnf, error) {
				return tt.cnf, nil
			}
			defer func() { loadDefault = orig }()
			t.Setenv("MYSQL_UNIX_PORT", tt.unixPort)
			t.Setenv("MYSQL_TCP_PORT", tt.tcpPort)
			t.Setenv("MYSQL_HOST", tt.host)
			t.Setenv("MYSQL_PWD", tt.pwd)

			got, err := loadConfig(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("unexpected config: (-want/+got):\n%s", diff)
			}
		})
	}
}
