package tests

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func withURLParams(req *http.Request, vars map[string]string) *http.Request {
	rc := chi.NewRouteContext()
	for k, v := range vars {
		rc.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	return req.WithContext(ctx)
}
