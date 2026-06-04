package views

import (
	"embed"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templateFS embed.FS

type Renderer struct {
	templates *template.Template
}

type LoginPageData struct {
	ErrorMessage string
	Username     string
}

type LoginSuccessData struct {
	Username string
}

func NewRenderer() (*Renderer, error) {
	templates, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Renderer{templates: templates}, nil
}

func (r *Renderer) RenderLoginPage(w io.Writer, data LoginPageData) error {
	return r.templates.ExecuteTemplate(w, "login.html", data)
}

func (r *Renderer) RenderLoginSuccess(w io.Writer, data LoginSuccessData) error {
	return r.templates.ExecuteTemplate(w, "success.html", data)
}
