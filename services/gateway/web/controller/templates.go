package controller

import (
	"encoding/json"
	"path"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	templates  = path.Join(basepath, "../", "templates")
	locales    = path.Join(basepath, "../", "locales")
	bundle     = i18n.NewBundle(language.English)
	editorPage = template.Must(template.ParseFiles(
		path.Join(templates, "editor.html"), path.Join(templates, "spinner.html"),
	))
	errorPage   = template.Must(template.ParseFiles(path.Join(templates, "error.html")))
	convertPage = template.Must(template.ParseFiles(
		path.Join(templates, "convert.html"), path.Join(templates, "error.html"), path.Join(templates, "spinner.html"),
	))
)

func init() {
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile(path.Join(locales, "en.json"))
	bundle.MustLoadMessageFile(path.Join(locales, "ru.json"))
}
