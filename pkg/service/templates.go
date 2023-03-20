package service

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	texttemplate "html/template"
	"io/fs"
	"os"
	"strings"

	"github.com/galdor/go-service/pkg/utils"
)

var templateFunctions = map[string]interface{}{}

func LoadTemplates(dirPath string) (*texttemplate.Template, *htmltemplate.Template, error) {
	textTemplate := texttemplate.New("")
	textTemplate.Option("missingkey=error")
	textTemplate.Funcs(templateFunctions)

	htmlTemplate := htmltemplate.New("")
	htmlTemplate.Option("missingkey=error")
	htmlTemplate.Funcs(templateFunctions)

	err := utils.WalkFS(dirPath,
		func(virtualPath, filePath string, info fs.FileInfo) error {
			isText := strings.HasSuffix(virtualPath, ".txt.gotpl")
			isHTML := strings.HasSuffix(virtualPath, ".html.gotpl")

			if !isText && !isHTML {
				return nil
			}

			start := len(dirPath) + 1
			end := len(virtualPath) - len(".gotpl")
			templateName := virtualPath[start:end]

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
