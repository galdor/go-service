package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"go.n16f.net/ejson"
	"gopkg.in/yaml.v3"
)

var cfgTemplateFuncs = map[string]interface{}{
	"env": os.Getenv,

	"quote": func(s string) string {
		data, _ := json.Marshal(s)
		return string(data)
	},

	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
}

func LoadCfg(filePath string, templateData, dest interface{}) error {
	baseData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}

	data, err := RenderCfg(filePath, baseData, templateData)
	if err != nil {
		return err
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

	if err := ejson.Unmarshal(jsonData, dest); err != nil {
		return fmt.Errorf("cannot decode json data: %w", err)
	}

	return nil
}

func RenderCfg(filePath string, templateContent []byte, templateData interface{}) ([]byte, error) {
	tpl := template.New(filepath.Base(filePath))
	tpl.Option("missingkey=error")
	tpl.Funcs(cfgTemplateFuncs)

	if _, err := tpl.Parse(string(templateContent)); err != nil {
		return nil, fmt.Errorf("cannot parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("cannot execute template: %w", err)
	}

	return buf.Bytes(), nil
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
