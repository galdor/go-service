package service

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"net/url"
	"os"
	"strings"
	texttemplate "text/template"

	"github.com/galdor/go-service/pkg/text"
	"github.com/galdor/go-service/pkg/utils"
)

var builtinTemplateFunctions = map[string]interface{}{
	"encodeURIQueryParameter": url.QueryEscape,

	"capitalize": text.Capitalize,
	"toSentence": text.ToSentence,
}

func LoadTemplates(dirPath string, templateFunctions map[string]interface{}) (*texttemplate.Template, *htmltemplate.Template, error) {
	textTemplate := texttemplate.New("")
	textTemplate.Option("missingkey=error")
	textTemplate.Funcs(builtinTemplateFunctions)
	textTemplate.Funcs(templateFunctions)

	htmlTemplate := htmltemplate.New("")
	htmlTemplate.Option("missingkey=error")
	htmlTemplate.Funcs(builtinTemplateFunctions)
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
