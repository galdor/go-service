package sjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPointerParse(t *testing.T) {
	assert := assert.New(t)

	assertParse := func(ep Pointer, s string) {
		t.Helper()

		var p Pointer
		if assert.NoError(p.Parse(s), s) {
			assert.Equal(ep, p, s)
		}
	}

	assertParse(Pointer{}, "")
	assertParse(Pointer{"foo"}, "/foo")
	assertParse(Pointer{"foo", "bar"}, "/foo/bar")
	assertParse(Pointer{"a", "b", "c"}, "/a/b/c")
	assertParse(Pointer{"xy", "", "z", "", ""}, "/xy//z//")
	assertParse(Pointer{"foo/bar", "~hello"}, "/foo~1bar/~0hello")
	assertParse(Pointer{"~1", "/0"}, "/~01/~10")
}

func TestPointerString(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("", Pointer{}.String())
	assert.Equal("/foo", Pointer{"foo"}.String())
	assert.Equal("/foo/bar", Pointer{"foo", "bar"}.String())
	assert.Equal("/a/b/c", Pointer{"a", "b", "c"}.String())
	assert.Equal("/xy//z//", Pointer{"xy", "", "z", "", ""}.String())
	assert.Equal("/foo~1bar/~0hello", Pointer{"foo/bar", "~hello"}.String())
	assert.Equal("/~01/~10", Pointer{"~1", "/0"}.String())
}

func TestPointerPrepend(t *testing.T) {
	assert := assert.New(t)

	p := NewPointer()

	p.Prepend("a")
	assert.Equal("/a", p.String())

	p.Prepend("b")
	assert.Equal("/b/a", p.String())

	p.Prepend("c", "d", "e")
	assert.Equal("/c/d/e/b/a", p.String())
}

func TestPointerAppend(t *testing.T) {
	assert := assert.New(t)

	p := NewPointer()

	p.Append("a")
	assert.Equal("/a", p.String())

	p.Append("b")
	assert.Equal("/a/b", p.String())

	p.Append("c", "d", "e")
	assert.Equal("/a/b/c/d/e", p.String())
}

func TestPointerParent(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("", Pointer{"a"}.Parent().String())
	assert.Equal("/a", Pointer{"a", "b"}.Parent().String())
	assert.Equal("/a/b", Pointer{"a", "b", "c"}.Parent().String())
}

func TestPointerChild(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("/a", Pointer{}.Child("a").String())
	assert.Equal("/a/b/c", Pointer{"a"}.Child("b", "c").String())
	assert.Equal("/a/b/c", Pointer{"a", "b"}.Child("c").String())
	assert.Equal("/a/1", Pointer{"a"}.Child(1).String())
	assert.Equal("/a/b/c", Pointer{"a"}.Child(Pointer{"b", "c"}).String())
}

func TestPointerFind(t *testing.T) {
	assert := assert.New(t)

	obj := map[string]interface{}{
		"a": 42,
		"b": map[string]interface{}{
			"x": 1,
		},
		"c": []interface{}{
			map[string]interface{}{
				"x": 2,
			},
			map[string]interface{}{
				"x": 3,
			},
		},
	}

	assert.Equal(obj,
		NewPointer().Find(obj))

	assert.Equal(nil,
		NewPointer("foo").Find(obj))

	assert.Equal(42,
		NewPointer("a").Find(obj))

	assert.Equal(map[string]interface{}{"x": 1},
		NewPointer("b").Find(obj))

	assert.Equal(nil,
		NewPointer("c", "foo").Find(obj))

	assert.Equal(nil,
		NewPointer("c", "-2").Find(obj))

	assert.Equal(nil,
		NewPointer("c", "3").Find(obj))

	assert.Equal(map[string]interface{}{"x": 2},
		NewPointer("c", "0").Find(obj))

	assert.Equal(2,
		NewPointer("c", "0", "x").Find(obj))

	assert.Equal(3,
		NewPointer("c", "1", "x").Find(obj))

	assert.Equal(nil,
		NewPointer("c", "1", "y").Find(obj))

	assert.Equal(nil,
		NewPointer("c", "1", "x", "2").Find(obj))
}
