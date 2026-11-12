package mycnf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		in   string
		want MyCnf
	}{
		{
			in:   "",
			want: MyCnf{},
		},
		{
			in:   "# comment\n",
			want: MyCnf{},
		},
		{
			in:   "; comment2\n",
			want: MyCnf{},
		},
		{
			in: "[group]\nkey=value\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "",
				},
			},
		},
		{
			in: "[group]\nkey= value \n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey = value\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey\t=\tvalue\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey   =   value\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\n key=value\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\n \t key \t = \t value \t \n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey \n",
			want: MyCnf{
				"group": map[string]string{
					"key": "",
				},
			},
		},
		{
			in: "[group]\nkey=\\n\\r\\t\\b\\s\\\"\\'\\\\\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "\n\r\t\b \"'\\",
				},
			},
		},
		{
			in: "[group]\nkey=\"\"\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "",
				},
			},
		},
		{
			in: "[group]\nkey=\"value\"\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey=\" 'value' \"\n",
			want: MyCnf{
				"group": map[string]string{
					"key": " 'value' ",
				},
			},
		},
		{
			in: "[group]\nkey=''\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "",
				},
			},
		},
		{
			in: "[group]\nkey='value'\n",
			want: MyCnf{
				"group": map[string]string{
					"key": "value",
				},
			},
		},
		{
			in: "[group]\nkey=' \"value\" '\n",
			want: MyCnf{
				"group": map[string]string{
					"key": " \"value\" ",
				},
			},
		},
		{
			in: "[group]\n" + `basedir="C:\Program Files\MySQL\MySQL Server 8.0"`,
			want: MyCnf{
				"group": map[string]string{
					"basedir": `C:\Program Files\MySQL\MySQL Server 8.0`,
				},
			},
		},
		{
			in: "[group]\n" + `basedir="C:\\Program Files\\MySQL\\MySQL Server 8.0"`,
			want: MyCnf{
				"group": map[string]string{
					"basedir": `C:\Program Files\MySQL\MySQL Server 8.0`,
				},
			},
		},
		{
			in: "[group]\n" + `basedir="C:/Program Files/MySQL/MySQL Server 8.0"`,
			want: MyCnf{
				"group": map[string]string{
					"basedir": `C:/Program Files/MySQL/MySQL Server 8.0`,
				},
			},
		},
		{
			in: "[group]\n" + `basedir=C:\\Program\sFiles\\MySQL\\MySQL\sServer\s8.0`,
			want: MyCnf{
				"group": map[string]string{
					"basedir": `C:\Program Files\MySQL\MySQL Server 8.0`,
				},
			},
		},

		// dashes and underscores are interchangeable, and the later one takes precedence.
		{
			in: "[client]\nssl-mode=DISABLED\nssl_mode=REQUIRED\n",
			want: MyCnf{
				"client": map[string]string{
					"ssl_mode": "REQUIRED",
				},
			},
		},
		{
			in: "[client]\nssl_mode=REQUIRED\nssl-mode=DISABLED\n",
			want: MyCnf{
				"client": map[string]string{
					"ssl-mode": "DISABLED",
				},
			},
		},
	}

	for _, tt := range tests {
		got, err := Unmarshal([]byte(tt.in))
		if err != nil {
			t.Errorf("input: %s, error: %v", tt.in, err)
			continue
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("input: %s, mismatch: (-want/+got)\n%s", tt.in, diff)
		}
	}
}
