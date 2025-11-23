// Package templates provides Go text templates for formatting validation messages.
package templates

import (
	"bytes"
	"strings"
	"text/template"
)

var funcMap = template.FuncMap{
	"join": strings.Join,
}

// Execute executes a template with the given data
func Execute(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MustExecute executes a template and panics on error
func MustExecute(tmpl *template.Template, data any) string {
	result, err := Execute(tmpl, data)
	if err != nil {
		panic(err)
	}
	return result
}

// Parse parses a template string with the funcMap
func Parse(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(funcMap).Parse(text))
}
