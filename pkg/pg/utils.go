package pg

import (
	"regexp"
	"strings"
)

var simpleIdentifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_$]*$`)

func QuoteString(s string) string {
	// (*pgx.Connection).QuoteString would do the job, but it requires a
	// connection object even though it does not use.

	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func QuoteIdentifier(s string) string {
	// (*pgx.Connection).QuoteIdentifier would also works, but 1/ it requires
	// a connection object and 2/ it quotes even when it is not necessary.

	if simpleIdentifierRe.MatchString(s) {
		return s
	}

	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
