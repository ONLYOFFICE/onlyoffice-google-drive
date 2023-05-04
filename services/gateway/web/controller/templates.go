package controller

import (
	"path"
	"path/filepath"
	"runtime"
	"text/template"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	templates  = path.Join(basepath, "../", "templates")
	editorPage = template.Must(template.ParseFiles(
		path.Join(templates, "editor.html"), path.Join(templates, "spinner.html"),
	))
	errorPage   = template.Must(template.ParseFiles(path.Join(templates, "error.html")))
	convertPage = template.Must(template.ParseFiles(
		path.Join(templates, "convert.html"), path.Join(templates, "error.html"), path.Join(templates, "spinner.html"),
	))
)
