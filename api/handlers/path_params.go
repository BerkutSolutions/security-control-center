package handlers

import (
	"net/http"
<<<<<<< HEAD
=======
	"strings"
>>>>>>> 2adc2fe (v1.0.5)

	"github.com/go-chi/chi/v5"
)

func urlParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func pathParams(r *http.Request) map[string]string {
	out := map[string]string{}
	rc := chi.RouteContext(r.Context())
<<<<<<< HEAD
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
=======
	if rc != nil {
		for i, key := range rc.URLParams.Keys {
			if i < len(rc.URLParams.Values) {
				out[key] = rc.URLParams.Values[i]
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	// Fallback for direct handler tests without chi route context.
	segments := strings.Split(strings.Trim(strings.TrimSpace(r.URL.Path), "/"), "/")
	addParamAfter(segments, "users", "id", out)
	addParamAfter(segments, "docs", "id", out)
	addParamAfter(segments, "incidents", "id", out)
	addParamAfter(segments, "stages", "stage_id", out)
	addParamAfter(segments, "links", "link_id", out)
	addParamAfter(segments, "attachments", "att_id", out)
	addParamAfter(segments, "blocks", "block_id", out)
	addParamAfter(segments, "files", "file_id", out)
	addParamAfter(segments, "artifacts", "artifact_id", out)
	return out
}

func addParamAfter(segments []string, marker, key string, out map[string]string) {
	if _, exists := out[key]; exists {
		return
	}
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == marker && strings.TrimSpace(segments[i+1]) != "" {
			out[key] = segments[i+1]
			return
		}
	}
}
>>>>>>> 2adc2fe (v1.0.5)
