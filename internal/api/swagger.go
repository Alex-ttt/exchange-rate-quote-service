package api

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// SwaggerUIHandler returns a handler for Swagger UI
func SwaggerUIHandler() http.HandlerFunc {
	return httpSwagger.WrapHandler
}

// OpenAPISpecHandler returns a handler that redirects to the swagger spec JSON
func OpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/doc.json", http.StatusTemporaryRedirect)
	}
}
