package service

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	texttemplate "html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func LoadTemplates(dirPath string) (*texttemplate.Template, *htmltemplate.Template, error) {
	// TODO funcmaps

	textTemplate := texttemplate.New("")
	textTemplate.Option("missingkey=error")

	htmlTemplate := htmltemplate.New("")
	htmlTemplate.Option("missingkey=error")

	err := filepath.Walk(dirPath,
		func(filePath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			isText := strings.HasSuffix(filePath, ".txt.gotpl")
			isHTML := strings.HasSuffix(filePath, ".html.gotpl")

			if !isText && !isHTML {
				return nil
			}

			start := len(dirPath) + 1
			end := len(filePath) - len(".gotpl")
			templateName := filePath[start:end]

			templateData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("cannot read %q: %w", filePath, err)
			}

			switch {
			case isText:
				template := textTemplate.New(templateName)
				_, err = template.Parse(string(templateData))

			case isHTML:
				template := htmlTemplate.New(templateName)
				_, err = template.Parse(string(templateData))
			}

			if err != nil {
				return fmt.Errorf("cannot parse %q: %w", filePath, err)
			}

			return nil
		})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, err
	}

	return textTemplate, htmlTemplate, nil
}
