package htmplx

import "html/template"

const (
	layoutTemplateString = `{{define "head"}}{{end}}
{{define "body"}}{{end}}
<!DOCTYPE html>
<html>
	<head>
		{{ template "head" . }}
	</head>

	<body>
		{{ template "body" . }}
	</body>
</html>`

	fragmentTemplateString = `{{define "body"}}{{end}}{{ template "body" . }}`
)

var (
	// ensure layout template is valid
	_ = template.Must(template.New("layout").Parse(layoutTemplateString))

	// ensure fragment template is valid
	_ = template.Must(template.New("fragment").Parse(fragmentTemplateString))
)
