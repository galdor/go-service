package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuoteString(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(`''`, QuoteString(``))
	assert.Equal(`'foo'`, QuoteString(`foo`))
	assert.Equal(`'foo bar'`, QuoteString(`foo bar`))
	assert.Equal(`'foo ''bar'''`, QuoteString(`foo 'bar'`))
	assert.Equal(`''''''`, QuoteString(`''`))
}

func TestQuoteIdentifier(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(`foo`, QuoteIdentifier(`foo`))
	assert.Equal(`"123foo"`, QuoteIdentifier(`123foo`))
	assert.Equal(`"foo ""bar"" baz"`, QuoteIdentifier(`foo "bar" baz`))
	assert.Equal(`"foo bar baz"`, QuoteIdentifier(`foo bar baz`))
	assert.Equal(`"""hello"""`, QuoteIdentifier(`"hello"`))
	assert.Equal(`""""""`, QuoteIdentifier(`""`))
}
