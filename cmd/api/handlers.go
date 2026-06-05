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
	cookie, err := r.Cookie("session_token")
	if err != nil {
		app.render(w, http.StatusOK, func(writer io.Writer) error {
			return app.renderer.RenderLoginPage(writer, views.LoginPageData{})
		})
		return
	}

	_, ok := app.authService.IsLogged(cookie.Value)
	if ok {
		app.render(w, http.StatusOK, func(writer io.Writer) error {
			return app.renderer.RenderHomePage(writer)
		})
	} else {
		app.render(w, http.StatusOK, func(writer io.Writer) error {
			return app.renderer.RenderLoginPage(writer, views.LoginPageData{})
		})
	}
}

func (app *application) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		_, ok := app.authService.IsLogged(cookie.Value)
		if ok {
			app.render(w, http.StatusOK, func(writer io.Writer) error {
				return app.renderer.RenderHomePage(writer)
			})
			return
		}
	}

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
				Username:     username,
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	metrics := app.metricsService.GetMetrics()

	err := app.writeJSON(w, 200, envelope{"data": metrics})
	if err != nil {
		app.errorResponse(w, 500, envelope{"error": err})
	}
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
