package api

import (
	"berkut-scc/core/appmeta"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"berkut-scc/tasks"
)

type ServerDeps struct {
	Users            store.UsersStore
	Sessions         store.SessionStore
	Roles            store.RolesStore
	Groups           store.GroupsStore
	Audits           store.AuditStore
	DocsStore        store.DocsStore
	ReportsStore     store.ReportsStore
	IncidentsStore   store.IncidentsStore
	ControlsStore    store.ControlsStore
	EntityLinksStore store.EntityLinksStore
	MonitoringStore  store.MonitoringStore
	AppHTTPSStore    store.AppHTTPSStore
	AppRuntimeStore  store.AppRuntimeStore
	UpdateChecker    *appmeta.UpdateChecker
	DashboardStore   store.DashboardStore
	DocsSvc          *docs.Service
	IncidentsSvc     *incidents.Service
	TasksStore       tasks.Store
	TasksSvc         *tasks.Service
	MonitoringEngine *monitoring.Engine
}
