package routegroups

import (
	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAssets(apiRouter chi.Router, g Guards, assets *handlers.AssetsHandler) {
	apiRouter.Route("/assets", func(assetsRouter chi.Router) {
		assetsRouter.MethodFunc("GET", "/", g.SessionPerm("assets.view", assets.List))
		assetsRouter.MethodFunc("GET", "/list", g.SessionPerm("assets.view", assets.ListLite))
		assetsRouter.MethodFunc("GET", "/export.csv", g.SessionPerm("assets.view", assets.ExportCSV))
		assetsRouter.MethodFunc("GET", "/autocomplete", g.SessionPerm("assets.view", assets.Autocomplete))
		assetsRouter.MethodFunc("POST", "/", g.SessionPerm("assets.manage", assets.Create))
		assetsRouter.MethodFunc("GET", "/{id:[0-9]+}", g.SessionPerm("assets.view", assets.Get))
		assetsRouter.MethodFunc("PUT", "/{id:[0-9]+}", g.SessionPerm("assets.manage", assets.Update))
		assetsRouter.MethodFunc("DELETE", "/{id:[0-9]+}", g.SessionPerm("assets.manage", assets.Archive))
		assetsRouter.MethodFunc("POST", "/{id:[0-9]+}/restore", g.SessionPerm("assets.manage", assets.Restore))

		assetsRouter.MethodFunc("GET", "/{id:[0-9]+}/software", g.SessionPerm("assets.view", assets.ListSoftware))
		assetsRouter.MethodFunc("POST", "/{id:[0-9]+}/software", g.SessionPerm("assets.manage", assets.AddSoftware))
		assetsRouter.MethodFunc("PUT", "/{id:[0-9]+}/software/{inst_id:[0-9]+}", g.SessionPerm("assets.manage", assets.UpdateSoftware))
		assetsRouter.MethodFunc("DELETE", "/{id:[0-9]+}/software/{inst_id:[0-9]+}", g.SessionPerm("assets.manage", assets.ArchiveSoftware))
		assetsRouter.MethodFunc("POST", "/{id:[0-9]+}/software/{inst_id:[0-9]+}/restore", g.SessionPerm("assets.manage", assets.RestoreSoftware))
	})
}
