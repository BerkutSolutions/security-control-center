package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func urlParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func pathParams(r *http.Request) map[string]string {
	out := map[string]string{}
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return out
	}
	for i, key := range rc.URLParams.Keys {
		if i < len(rc.URLParams.Values) {
			out[key] = rc.URLParams.Values[i]
		}
	}
	return out
}
