package routegroups

import (
	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterFindings(apiRouter chi.Router, g Guards, findings *handlers.FindingsHandler) {
	apiRouter.Route("/findings", func(findingsRouter chi.Router) {
		findingsRouter.MethodFunc("GET", "/", g.SessionPerm("findings.view", findings.List))
		findingsRouter.MethodFunc("GET", "/list", g.SessionPerm("findings.view", findings.ListLite))
		findingsRouter.MethodFunc("GET", "/export.csv", g.SessionPerm("findings.view", findings.ExportCSV))
		findingsRouter.MethodFunc("GET", "/autocomplete", g.SessionPerm("findings.view", findings.Autocomplete))
		findingsRouter.MethodFunc("POST", "/", g.SessionPerm("findings.manage", findings.Create))
		findingsRouter.MethodFunc("GET", "/{id:[0-9]+}", g.SessionPerm("findings.view", findings.Get))
		findingsRouter.MethodFunc("PUT", "/{id:[0-9]+}", g.SessionPerm("findings.manage", findings.Update))
		findingsRouter.MethodFunc("DELETE", "/{id:[0-9]+}", g.SessionPerm("findings.manage", findings.Archive))
		findingsRouter.MethodFunc("POST", "/{id:[0-9]+}/restore", g.SessionPerm("findings.manage", findings.Restore))

		findingsRouter.MethodFunc("GET", "/{id:[0-9]+}/links", g.SessionPerm("findings.view", findings.ListLinks))
		findingsRouter.MethodFunc("POST", "/{id:[0-9]+}/links", g.SessionPerm("findings.manage", findings.AddLink))
		findingsRouter.MethodFunc("DELETE", "/{id:[0-9]+}/links/{link_id:[0-9]+}", g.SessionPerm("findings.manage", findings.DeleteLink))
	})
}
