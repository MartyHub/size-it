package server

import (
	"embed"
	"html/template"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	rootEmbed = "templates"
	rootLive  = "internal/server/templates"

	pathComponents = "components"
	pathViews      = "views"

	templateLayout = "layout.gohtml"
)

//go:embed templates/*
var templates embed.FS

type (
	embedRenderer struct {
		tpls map[string]*template.Template
	}

	liveRenderer struct{}
)

func newEmbedRenderer() *embedRenderer {
	res := &embedRenderer{tpls: make(map[string]*template.Template)}

	entries, err := templates.ReadDir(path.Join(rootEmbed, pathViews))
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		slog.Info("Parsing view's templates...", slog.String("view", entry.Name()))

		res.tpls[entry.Name()] = template.Must(template.ParseFS(
			templates,
			path.Join(rootEmbed, templateLayout),
			path.Join(rootEmbed, pathViews, entry.Name()),
			path.Join(rootEmbed, pathViews, pathComponents, "*"),
		))
	}

	return res
}

func (rdr *embedRenderer) Render(w io.Writer, view string, data any, _ echo.Context) error {
	return rdr.tpls[view].Execute(w, data)
}

func newLiveRenderer() *liveRenderer {
	return &liveRenderer{}
}

func (rdr *liveRenderer) Render(w io.Writer, name string, data any, _ echo.Context) error {
	var (
		tpl *template.Template
		err error
	)

	if strings.HasPrefix(name, pathComponents) {
		tpl, err = template.ParseFS(os.DirFS(rootLive), path.Join(pathViews, name))
	} else {
		tpl, err = template.ParseFS(os.DirFS(rootLive),
			templateLayout,
			path.Join(pathViews, name),
			path.Join(pathViews, pathComponents, "*"),
		)
	}

	if err != nil {
		return err
	}

	return tpl.Execute(w, data)
}
