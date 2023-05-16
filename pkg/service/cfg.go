package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	jsonvalidator "github.com/galdor/go-json-validator"
	"gopkg.in/yaml.v3"
)

func LoadCfg(filePath string, dest interface{}) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}

	yamlDecoder := yaml.NewDecoder(bytes.NewReader(data))

	var yamlValue interface{}
	if err := yamlDecoder.Decode(&yamlValue); err != nil && err != io.EOF {
		return fmt.Errorf("cannot decode yaml data: %w", err)
	}

	jsonValue, err := YAMLValueToJSONValue(yamlValue)
	if err != nil {
		return fmt.Errorf("invalid yaml data: %w", err)
	}

	jsonData, err := json.Marshal(jsonValue)
	if err != nil {
		return fmt.Errorf("cannot generate json data: %w", err)
	}

	if err := jsonvalidator.Unmarshal(jsonData, dest); err != nil {
		return fmt.Errorf("cannot decode json data: %w", err)
	}

	return nil
}

func YAMLValueToJSONValue(yamlValue interface{}) (interface{}, error) {
	// For some reason, go-yaml will return objects as map[string]interface{}
	// if all keys are strings, and as map[interface{}]interface{} if not. So
	// we have to handle both.

	var jsonValue interface{}

	switch v := yamlValue.(type) {
	case []interface{}:
		array := make([]interface{}, len(v))

		for i, yamlElement := range v {
			jsonElement, err := YAMLValueToJSONValue(yamlElement)
			if err != nil {
				return nil, err
			}

			array[i] = jsonElement
		}

		jsonValue = array

	case map[interface{}]interface{}:
		object := make(map[string]interface{})

		for key, yamlEntry := range v {
			keyString, ok := key.(string)
			if !ok {
				return nil,
					fmt.Errorf("object key \"%v\" is not a string", key)
			}

			jsonEntry, err := YAMLValueToJSONValue(yamlEntry)
			if err != nil {
				return nil, err
			}

			object[keyString] = jsonEntry
		}

		jsonValue = object

	case map[string]interface{}:
		object := make(map[string]interface{})

		for key, yamlEntry := range v {
			jsonEntry, err := YAMLValueToJSONValue(yamlEntry)
			if err != nil {
				return nil, err
			}

			object[key] = jsonEntry
		}

		jsonValue = object

	default:
		jsonValue = yamlValue
	}

	return jsonValue, nil
}
