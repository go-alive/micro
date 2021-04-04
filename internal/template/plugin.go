package template

var (
	Plugin = `package main
{{if .Plugins}}
import ({{range .Plugins}}
	_ "github.com/go-alive/go-plugins/{{.}}"{{end}}
){{end}}
`
)
