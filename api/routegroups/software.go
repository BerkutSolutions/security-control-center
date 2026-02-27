package routegroups

import (
	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterSoftware(apiRouter chi.Router, g Guards, software *handlers.SoftwareHandler) {
	apiRouter.Route("/software", func(r chi.Router) {
		r.MethodFunc("GET", "/", g.SessionPerm("software.view", software.List))
		r.MethodFunc("GET", "/list", g.SessionPerm("software.view", software.ListLite))
		r.MethodFunc("GET", "/export.csv", g.SessionPerm("software.view", software.ExportCSV))
		r.MethodFunc("GET", "/autocomplete", g.SessionPerm("software.view", software.Autocomplete))
		r.MethodFunc("POST", "/", g.SessionPerm("software.manage", software.Create))
		r.MethodFunc("GET", "/{id:[0-9]+}", g.SessionPerm("software.view", software.Get))
		r.MethodFunc("PUT", "/{id:[0-9]+}", g.SessionPerm("software.manage", software.Update))
		r.MethodFunc("DELETE", "/{id:[0-9]+}", g.SessionPerm("software.manage", software.Archive))
		r.MethodFunc("POST", "/{id:[0-9]+}/restore", g.SessionPerm("software.manage", software.Restore))

		r.MethodFunc("GET", "/{id:[0-9]+}/versions", g.SessionPerm("software.view", software.ListVersions))
		r.MethodFunc("POST", "/{id:[0-9]+}/versions", g.SessionPerm("software.manage", software.CreateVersion))
		r.MethodFunc("PUT", "/{id:[0-9]+}/versions/{version_id:[0-9]+}", g.SessionPerm("software.manage", software.UpdateVersion))
		r.MethodFunc("DELETE", "/{id:[0-9]+}/versions/{version_id:[0-9]+}", g.SessionPerm("software.manage", software.ArchiveVersion))
		r.MethodFunc("POST", "/{id:[0-9]+}/versions/{version_id:[0-9]+}/restore", g.SessionPerm("software.manage", software.RestoreVersion))

		r.MethodFunc("GET", "/{id:[0-9]+}/assets", g.SessionPerm("software.view", software.ListProductAssets))
	})
}
