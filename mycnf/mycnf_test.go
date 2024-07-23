package mycnf

import (
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_listConfigureFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		test_listConfigureFile_windows(t)
	} else {
		test_listConfigureFile_unix(t)
	}
}

func test_listConfigureFile_windows(t *testing.T) {
	t.Run("no extra file", func(t *testing.T) {
		t.Setenv("WINDIR", `C:\Windows`)
		got := listConfigureFile("")
		want := []string{`C:\Windows\my.ini`, `C:\Windows\my.cnf`, `C:\my.ini`, `C:\my.cnf`}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("unexpected paths (-got +want):\n%s", diff)
		}
	})

	t.Run("with extra file", func(t *testing.T) {
		t.Setenv("HOME", "/home/user")
		got := listConfigureFile("extra.cnf")
		want := []string{`C:\Windows\my.ini`, `C:\Windows\my.cnf`, `C:\my.ini`, `C:\my.cnf`, "extra.cnf"}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("unexpected paths (-got +want):\n%s", diff)
		}
	})
}

func test_listConfigureFile_unix(t *testing.T) {
	t.Run("no extra file", func(t *testing.T) {
		t.Setenv("HOME", "/home/user")
		got := listConfigureFile("")
		want := []string{"/etc/my.cnf", "/etc/mysql/my.cnf", "/home/user/.my.cnf"}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("unexpected paths (-got +want):\n%s", diff)
		}
	})

	t.Run("with extra file", func(t *testing.T) {
		t.Setenv("HOME", "/home/user")
		got := listConfigureFile("extra.cnf")
		want := []string{"/etc/my.cnf", "/etc/mysql/my.cnf", "extra.cnf", "/home/user/.my.cnf"}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("unexpected paths (-got +want):\n%s", diff)
		}
	})
}

func TestLoad(t *testing.T) {
	cnf, err := load([]string{"testdata/global.cnf", "testdata/not-exists.cnf", "testdata/user.cnf"})
	if err != nil {
		t.Fatal(err)
	}
	want := MyCnf{
		"client": {
			"port":     "3306",
			"socket":   "/tmp/mysql.sock",
			"password": "my password",
		},
		"mysql": {
			"no-auto-rehash":  "",
			"connect_timeout": "2",
		},
		"mysqld": {
			"port":               "3306",
			"socket":             "/tmp/mysql.sock",
			"key_buffer_size":    "16M",
			"max_allowed_packet": "128M",
		},
		"mysqldump": {
			"quick": "",
		},
	}
	if diff := cmp.Diff(cnf, want); diff != "" {
		t.Errorf("unexpected result (-got +want):\n%s", diff)
	}
}
