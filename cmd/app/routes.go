package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"quoteservice/internal/api"
	"quoteservice/internal/api/middleware"
	"quoteservice/internal/service"
)

func (app *App) initHTTP(quoteService service.QuoteServiceInterface) {
	r := chi.NewRouter()
	r.Use(middleware.RequestIDMiddleware)
	r.Use(middleware.RequestLoggingMiddleware(app.logger))
	r.Use(chimiddleware.Recoverer)

	r.Post("/quotes/update", api.HandleRequestUpdate(quoteService))
	r.Get("/quotes/{update_id}", api.HandleGetQuoteByID(quoteService))
	r.Get("/quotes/latest", api.HandleGetLatestQuote(quoteService))
	r.Get("/healthz", api.HandleHealthz())
	r.Get("/readyz", api.HandleReadyz(app.db, app.rdb))

	if app.cfg.Server.ServeSwagger {
		r.Get("/swagger/*", api.SwaggerUIHandler())
		r.Get("/openapi.json", api.OpenAPISpecHandler())
	}

	app.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", app.cfg.Server.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
