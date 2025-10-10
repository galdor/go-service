package stemplate

import (
	"encoding/json"
	"os"
	"strings"
)

var Funcs = map[string]interface{}{
	"env": os.Getenv,

	"quote": func(s string) string {
		data, _ := json.Marshal(s)
		return string(data)
	},

	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
}
