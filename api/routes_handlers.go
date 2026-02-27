package api

import "berkut-scc/api/handlers"

type routeHandlers struct {
	auth        *handlers.AuthHandler
	accounts    *handlers.AccountsHandler
	dashboard   *handlers.DashboardHandler
	placeholder *handlers.PlaceholderHandler
	settings    *handlers.SettingsHandler
	https       *handlers.HTTPSSettingsHandler
	runtime     *handlers.RuntimeSettingsHandler
	hardening   *handlers.HardeningHandler
	docs        *handlers.DocsHandler
	reports     *handlers.ReportsHandler
	incidents   *handlers.IncidentsHandler
	controls    *handlers.ControlsHandler
	assets      *handlers.AssetsHandler
	findings    *handlers.FindingsHandler
	software    *handlers.SoftwareHandler
	logs        *handlers.LogsHandler
	monitoring  *handlers.MonitoringHandler
}

func (s *Server) newRouteHandlers() routeHandlers {
	return routeHandlers{
		auth:        handlers.NewAuthHandler(s.cfg, s.users, s.sessions, s.incidentsStore, s.sessionManager, s.policy, s.audits, s.logger),
		accounts:    handlers.NewAccountsHandler(s.users, s.groups, s.roles, s.sessions, s.policy, s.sessionManager, s.cfg, s.audits, s.logger, s.refreshPolicy),
		dashboard:   handlers.NewDashboardHandler(s.cfg, s.dashboardStore, s.users, s.docsStore, s.incidentsStore, s.docsSvc, s.incidentsSvc, s.tasksStore, s.audits, s.policy, s.logger),
		placeholder: handlers.NewPlaceholderHandler(),
		settings:    handlers.NewSettingsHandler(),
		https:       handlers.NewHTTPSSettingsHandler(s.cfg, s.appHTTPSStore, s.audits),
		runtime:     handlers.NewRuntimeSettingsHandler(s.cfg, s.appRuntimeStore, s.updateChecker, s.audits),
		hardening:   handlers.NewHardeningHandler(s.cfg, s.appHTTPSStore, s.appRuntimeStore, s.audits),
		docs:        handlers.NewDocsHandler(s.cfg, s.docsStore, s.entityLinksStore, s.controlsStore, s.assetsStore, s.softwareStore, s.users, s.policy, s.docsSvc, s.audits, s.logger),
		reports:     handlers.NewReportsHandler(s.cfg, s.docsStore, s.reportsStore, s.users, s.policy, s.docsSvc, s.incidentsStore, s.incidentsSvc, s.controlsStore, s.monitoringStore, s.tasksSvc, s.audits, s.logger),
		incidents:   handlers.NewIncidentsHandler(s.cfg, s.incidentsStore, s.entityLinksStore, s.controlsStore, s.assetsStore, s.softwareStore, s.users, s.docsStore, s.policy, s.incidentsSvc, s.docsSvc, s.audits, s.logger),
		controls:    handlers.NewControlsHandler(s.controlsStore, s.entityLinksStore, s.users, s.docsStore, s.incidentsStore, s.tasksStore, s.assetsStore, s.softwareStore, s.audits, s.policy, s.logger),
		assets:      handlers.NewAssetsHandler(s.assetsStore, s.softwareStore, s.users, s.audits, s.policy),
		findings:    handlers.NewFindingsHandler(s.findingsStore, s.entityLinksStore, s.users, s.assetsStore, s.controlsStore, s.softwareStore, s.audits, s.policy),
		software:    handlers.NewSoftwareHandler(s.softwareStore, s.users, s.assetsStore, s.audits, s.policy),
		logs:        handlers.NewLogsHandler(s.audits),
		monitoring:  handlers.NewMonitoringHandler(s.monitoringStore, s.users, s.audits, s.monitoringEngine, s.policy, s.incidentsSvc.Encryptor()),
	}
}
