package utils

import (
	"fmt"
	"os"
	"runtime"
)

func Abort(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func Panicf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func RecoverValueData(value interface{}) (msg, trace string) {
	switch v := value.(type) {
	case error:
		msg = v.Error()
	case string:
		msg = v
	default:
		msg = fmt.Sprintf("%#v", v)
	}

	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	buf = buf[0 : n-1]
	trace = string(buf)

	return
}
