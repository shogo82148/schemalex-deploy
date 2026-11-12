package mycnf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type MyCnf map[string]map[string]string

// LoadDefault loads my.cnf from the default path.
func LoadDefault(extraFile string) (MyCnf, error) {
	paths := listConfigureFile(extraFile)
	return load(paths)
}

// listConfigureFile returns a list of paths to read my.cnf.
func listConfigureFile(extraFile string) []string {
	// https://dev.mysql.com/doc/refman/8.0/en/option-files.html
	var paths []string
	if runtime.GOOS == "windows" {
		// Option Files Read on Windows Systems
		if windir := os.Getenv("WINDIR"); windir != "" {
			paths = append(paths, filepath.Join(windir, "my.ini"), filepath.Join(windir, "my.cnf"))
		}
		paths = append(paths, `C:\my.ini`, `C:\my.cnf`)
		if extraFile != "" {
			paths = append(paths, extraFile)
		}
	} else {
		// Option Files Read on Unix and Unix-Like Systems
		paths = append(paths, "/etc/my.cnf", "/etc/mysql/my.cnf")
		if extraFile != "" {
			paths = append(paths, extraFile)
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			paths = append(paths, filepath.Join(home, ".my.cnf"))
		}
	}
	return paths
}

func load(paths []string) (MyCnf, error) {
	result := MyCnf{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		cnf, err := Unmarshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %w", p, err)
		}

		for group, kv := range cnf {
			g, ok := result[group]
			if !ok {
				g = map[string]string{}
				result[group] = g
			}
			for name, value := range kv {
				setOption(g, name, value)
			}
		}
	}
	return result, nil
}

// setOption sets the option to the group.
// mysql(1) treats dashes and underscores in option names interchangeably,
// and the option that appears later takes precedence.
// So setOption removes the same option written with the other separator,
// to make the later one take precedence regardless of how it is written.
// https://dev.mysql.com/doc/refman/8.0/en/option-files.html
func setOption(group map[string]string, name, value string) {
	normalized := normalizeOptionName(name)
	for k := range group {
		if k != name && normalizeOptionName(k) == normalized {
			delete(group, k)
		}
	}
	group[name] = value
}

func normalizeOptionName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}
