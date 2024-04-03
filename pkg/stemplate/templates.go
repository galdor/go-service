package stemplate

import (
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
)

func LoadTextTemplates(dirPath string, funcMap texttemplate.FuncMap) (*texttemplate.Template, error) {
	rootTpl := texttemplate.New("")

	rootTpl = rootTpl.Option("missingkey=error")
	rootTpl = rootTpl.Funcs(funcMap)

	err := filepath.Walk(dirPath,
		func(filePath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if !strings.HasSuffix(filePath, ".txt.gotpl") {
				return nil
			}

			start := len(dirPath) + 1
			end := len(filePath) - len(".gotpl")
			tplName := filePath[start:end]

			tplData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("cannot read %q: %w", filePath, err)
			}

			tmpl := rootTpl.New(tplName)
			if _, err := tmpl.Parse(string(tplData)); err != nil {
				return fmt.Errorf("cannot parse %q: %w", filePath, err)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return rootTpl, nil
}

func LoadHTMLTemplates(dirPath string, funcMap htmltemplate.FuncMap) (*htmltemplate.Template, error) {
	rootTpl := htmltemplate.New("")

	rootTpl = rootTpl.Option("missingkey=error")
	rootTpl = rootTpl.Funcs(funcMap)

	err := filepath.Walk(dirPath,
		func(filePath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if !strings.HasSuffix(filePath, ".html.gotpl") {
				return nil
			}

			start := len(dirPath) + 1
			end := len(filePath) - len(".gotpl")
			tplName := filePath[start:end]

			tplData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("cannot read %q: %w", filePath, err)
			}

			tmpl := rootTpl.New(tplName)
			if _, err := tmpl.Parse(string(tplData)); err != nil {
				return fmt.Errorf("cannot parse %q: %w", filePath, err)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return rootTpl, nil
}
