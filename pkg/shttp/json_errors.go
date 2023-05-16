package shttp

import (
	"encoding/json"
	"fmt"

	jsonvalidator "github.com/galdor/go-json-validator"
)

// JSONError is the error structure used by all web and API HTTP servers. It
// is a go-service convention.
//
// We keep a raw copy of the error data object so that it can be decoded to a
// specific Go type if necessary.

type JSONError struct {
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message"`
	RawData json.RawMessage `json:"data,omitempty"`
	Data    ErrorData       `json:"-"`
}

type ValidationJSONErrorData struct {
	ValidationErrors jsonvalidator.ValidationErrors `json:"validationErrors"`
}

func (err *JSONError) MarshalJSON() ([]byte, error) {
	type JSONError2 JSONError

	err2 := JSONError2(*err)

	if err2.Data != nil {
		var dataErr error

		err2.RawData, dataErr = json.Marshal(err2.Data)
		if dataErr != nil {
			return nil, fmt.Errorf("cannot encode error data: %w", dataErr)
		}
	}

	return json.Marshal(err2)
}

func (err *JSONError) UnmarshalJSON(data []byte) error {
	type JSONError2 JSONError

	err2 := JSONError2(*err)

	if err3 := json.Unmarshal(data, &err2); err3 != nil {
		return err3
	}

	if err2.RawData != nil {
		dataErr := json.Unmarshal(err2.RawData, &err2.Data)
		if dataErr != nil {
			return fmt.Errorf("cannot decode error data: %w", dataErr)
		}
	}

	*err = JSONError(err2)

	return nil
}

func (err *JSONError) Error() string {
	if err.Code == "" {
		return err.Message
	} else {
		return fmt.Sprintf("%s: %s", err.Code, err.Message)
	}
}

func (err *JSONError) DecodeData(target interface{}) error {
	if err.RawData == nil {
		return fmt.Errorf("missing or empty error data")
	}

	return json.Unmarshal(err.RawData, target)
}
