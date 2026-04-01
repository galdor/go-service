package stemplate

import (
	"cmp"
	"encoding/json"
	"os"
	"strings"
)

var Funcs = map[string]interface{}{
	"env": os.Getenv,

	"env2": func(name, defaultValue string) string {
		return cmp.Or(os.Getenv(name), defaultValue)
	},

	"quote": func(s string) string {
		data, _ := json.Marshal(s)
		return string(data)
	},

	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
}
