package mycnf

import "testing"

func FuzzUnmarshal(f *testing.F) {
	f.Add("")
	f.Add("# comment\n")
	f.Add("; comment2\n")
	f.Add("[group]\nkey=value\n")
	f.Add("[group]\nkey\n")
	f.Add("[group]\nkey= value \n")
	f.Add("[group]\nkey=\\n\\r\\t\\b\\s\\\"\\'\\\\\n")
	f.Add("[group]\nkey=\"\"\n")
	f.Add("[group]\nkey=\"value\"\n")
	f.Add("[group]\nkey=\" 'value' \"\n")
	f.Add("[group]\nkey=''\n")
	f.Add("[group]\nkey='value'\n")
	f.Add("[group]\nkey=' \"value\" '\n")
	f.Add("[group]\n" + `basedir="C:\Program Files\MySQL\MySQL Server 8.0"`)
	f.Add("[group]\n" + `basedir="C:\\Program Files\\MySQL\\MySQL Server 8.0"`)
	f.Add("[group]\n" + `basedir="C:/Program Files/MySQL/MySQL Server 8.0"`)
	f.Add("[group]\n" + `basedir=C:\\Program\sFiles\\MySQL\\MySQL\sServer\s8.0`)

	f.Fuzz(func(t *testing.T, conf string) {
		_, err := Unmarshal([]byte(conf))
		if err != nil {
			return
		}
	})
}
