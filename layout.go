package htmplx

import "html/template"

const (
	layoutTemplateString = `{{define "head"}}{{end}}
{{define "body"}}{{end}}
<DOCTYPE html>
<html>
	<head>
		{{ template "head" . }}
	</head>

	<body>
		{{ template "body" . }}
	</body>
</html>`
)

var (
	layoutTemplate = template.Must(template.New("layout").Parse(layoutTemplateString))
)
