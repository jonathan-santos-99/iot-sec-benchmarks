package views

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templateFS embed.FS

const layoutFile = "templates/layout.html"

var pages = map[string]string{
	"login.html": "templates/login.html",
	"home.html":  "templates/home.html",
}

type Renderer struct {
	templates map[string]*template.Template
}

type LoginPageData struct {
	ErrorMessage string
	Username     string
}

func NewRenderer() (*Renderer, error) {
	templates := make(map[string]*template.Template, len(pages))
	for name, path := range pages {
		t, err := template.ParseFS(templateFS, layoutFile, path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		templates[name] = t
	}
	return &Renderer{templates: templates}, nil
}

func (r *Renderer) render(w io.Writer, name string, data any) error {
	t, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %q não encontrado", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func (r *Renderer) RenderLoginPage(w io.Writer, data LoginPageData) error {
	return r.render(w, "login.html", data)
}

func (r *Renderer) RenderHomePage(w io.Writer) error {
	return r.render(w, "home.html", nil)
}
