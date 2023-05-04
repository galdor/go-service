package sjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	jsonpointer "github.com/galdor/go-json-pointer"
)

// It would be nice to have a DecodeStrict() function which would use
// (*json.Decoder).DisallowUnknownFields(). Infortunately, the errors produced
// this way are not structured and therefore unusable. There is no way out
// this whole mess without rewriting an unmarshaller based on
// (*json.Decoder).Token(). This would increase memory pressure, but this is
// irrelevant for most use cases and would allow much better error reporting.

func Decode(r io.Reader, dest interface{}) error {
	d := json.NewDecoder(r)
	return decode(d, dest)
}

func DecodeData(data []byte, dest interface{}) error {
	return Decode(bytes.NewReader(data), dest)
}

func decode(d *json.Decoder, dest interface{}) error {
	if err := d.Decode(dest); err != nil {
		return ConvertUnmarshallingError(err)
	}

	return Validate(dest)
}

func ConvertUnmarshallingError(err error) error {
	switch err2 := err.(type) {
	case *json.UnmarshalTypeError:
		var pointer jsonpointer.Pointer

		if err2.Field != "" {
			parts := strings.Split(err2.Field, ".")
			pointer = jsonpointer.NewPointer(parts...)
		}

		message := fmt.Sprintf("cannot decode %v into value of type %v",
			err2.Value, err2.Type)

		return &ValidationError{
			Pointer: pointer,
			Code:    "invalidValueType",
			Message: message,
		}

	default:
		return err
	}
}
