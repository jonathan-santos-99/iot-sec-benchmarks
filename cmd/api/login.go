package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"

	"fishSim/views"
)

func (app *application) handleHomePage(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("session_token")
	username := app.sessions[cookie.Value]
	app.render(w, http.StatusOK, func(writer io.Writer) error {
		return app.renderer.RenderLoginSuccess(writer, views.LoginSuccessData{Username: username})
	})
}

func (app *application) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, http.StatusOK, func(writer io.Writer) error {
		return app.renderer.RenderLoginPage(writer, views.LoginPageData{})
	})
}

func (app *application) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "não foi possível ler o formulário", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	token, ok := app.authService.CreateSessionCookie(username, password)
	if !ok {
		app.render(w, http.StatusOK, func(writer io.Writer) error {
			return app.renderer.RenderLoginPage(writer, views.LoginPageData{
				ErrorMessage: "Credenciais inválidas. Tente novamente.",
			})
		})
		return
	}

	log.Printf("Usuario %s autenticado", username)

	app.sessions[token] = username
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)

	app.render(w, http.StatusOK, func(writer io.Writer) error {
		return app.renderer.RenderLoginSuccess(writer, views.LoginSuccessData{Username: username})
	})
}

func (app *application) render(w http.ResponseWriter, statusCode int, renderFn func(io.Writer) error) {
	var html bytes.Buffer
	if err := renderFn(&html); err != nil {
		http.Error(w, "erro ao renderizar a página", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = w.Write(html.Bytes())
}
