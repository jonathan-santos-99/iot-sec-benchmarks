package main

import (
	"fishSim/views"
	"io"
	"net/http"
)

func (app *application) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			app.render(w, http.StatusOK, func(writer io.Writer) error {
				return app.renderer.RenderLoginPage(writer, views.LoginPageData{})
			})
			return
		}

		_, ok := app.sessions[cookie.Value]
		if !ok {
			app.render(w, http.StatusUnauthorized, func(writer io.Writer) error {
				return app.renderer.RenderLoginPage(writer, views.LoginPageData{})
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
