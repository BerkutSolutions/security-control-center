package api

import (
	"context"
	"database/sql"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/appmeta"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/tasks"
	taskhttp "berkut-scc/tasks/http"
	taskstore "berkut-scc/tasks/store"
	"berkut-scc/core/utils"
	"berkut-scc/gui"
	"github.com/gorilla/mux"
)

type Server struct {
	cfg            *config.AppConfig
	router         *mux.Router
	httpServer     *http.Server
	logger         *utils.Logger
	sessionManager *auth.SessionManager
	users          store.UsersStore
	sessions       store.SessionStore
	roles          store.RolesStore
	groups         store.GroupsStore
	audits         store.AuditStore
	policy         *rbac.Policy
	docsStore      store.DocsStore
	reportsStore   store.ReportsStore
	docsSvc        *docs.Service
	incidentsStore store.IncidentsStore
	incidentsSvc   *incidents.Service
	tasksStore     tasks.Store
	tasksSvc       *tasks.Service
	tasksScheduler *tasks.RecurringScheduler
	dashboardStore store.DashboardStore
	controlsStore  store.ControlsStore
	entityLinksStore store.EntityLinksStore
	monitoringStore store.MonitoringStore
	appHTTPSStore store.AppHTTPSStore
	appRuntimeStore store.AppRuntimeStore
	updateChecker *appmeta.UpdateChecker
	monitoringEngine *monitoring.Engine
	activityTracker *sessionActivity
}

func NewServer(cfg *config.AppConfig, db *sql.DB, logger *utils.Logger) *Server {
	ensureMimeTypes()
	users := store.NewUsersStore(db)
	sessions := store.NewSessionsStore(db)
	roles := store.NewRolesStore(db)
	groups := store.NewGroupsStore(db)
	audits := store.NewAuditStore(db)
	docsStore := store.NewDocsStore(db)
	reportsStore := store.NewReportsStore(db)
	incidentsStore := store.NewIncidentsStore(db)
	controlsStore := store.NewControlsStore(db)
	entityLinksStore := store.NewEntityLinksStore(db)
	monitoringStore := store.NewMonitoringStore(db)
	appHTTPSStore := store.NewAppHTTPSStore(db)
	appRuntimeStore := store.NewAppRuntimeStore(db)
	updateChecker := appmeta.NewUpdateChecker()
	tasksStore := taskstore.NewSQLite(db)
	dashboardStore := store.NewDashboardStore(db)
	docsSvc, err := docs.NewService(cfg, docsStore, users, audits, logger)
	if err != nil && logger != nil {
		logger.Fatalf("docs service init: %v", err)
	}
	incidentsSvc, err := incidents.NewService(cfg, audits)
	if err != nil && logger != nil {
		logger.Fatalf("incidents service init: %v", err)
	}
	tasksSvc := tasks.NewService(tasksStore)
	tasksScheduler := tasks.NewRecurringScheduler(cfg.Scheduler, tasksSvc.Store(), audits, logger)
	monitoringEngine := monitoring.NewEngineWithDeps(monitoringStore, incidentsStore, audits, cfg.Incidents.RegNoFormat, incidentsSvc.Encryptor(), monitoring.NewHTTPTelegramSender(), logger)
	s := &Server{
		cfg:            cfg,
		router:         mux.NewRouter(),
		logger:         logger,
		sessionManager: auth.NewSessionManager(sessions, cfg, logger),
		users:          users,
		sessions:       sessions,
		roles:          roles,
		groups:         groups,
		audits:         audits,
		policy:         rbac.NewPolicy(nil),
		docsStore:      docsStore,
		reportsStore:   reportsStore,
		docsSvc:        docsSvc,
		incidentsStore: incidentsStore,
		incidentsSvc:   incidentsSvc,
		controlsStore:  controlsStore,
		entityLinksStore: entityLinksStore,
		monitoringStore: monitoringStore,
		appHTTPSStore: appHTTPSStore,
		appRuntimeStore: appRuntimeStore,
		updateChecker: updateChecker,
		monitoringEngine: monitoringEngine,
		tasksStore:     tasksStore,
		tasksSvc:       tasksSvc,
		tasksScheduler: tasksScheduler,
		dashboardStore: dashboardStore,
		activityTracker: newSessionActivity(),
	}
	if err := s.bootstrapRoles(context.Background()); err != nil && logger != nil {
		logger.Errorf("bootstrap roles: %v", err)
	}
	s.registerRoutes()
	return s
}

func (s *Server) Start() error {
	if s.tasksScheduler != nil {
		s.tasksScheduler.Start()
	}
	if s.monitoringEngine != nil {
		s.monitoringEngine.Start()
	}
	s.httpServer = &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      s.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if s.cfg.TLSEnabled {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.tasksScheduler != nil {
		s.tasksScheduler.Stop()
	}
	if s.monitoringEngine != nil {
		s.monitoringEngine.Stop()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.securityHeadersMiddleware)
	staticHandler := s.staticHandler()
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticHandler))

	authHandler := handlers.NewAuthHandler(s.cfg, s.users, s.sessions, s.sessionManager, s.policy, s.audits, s.logger)
	accHandler := handlers.NewAccountsHandler(s.users, s.groups, s.roles, s.sessions, s.policy, s.sessionManager, s.cfg, s.audits, s.logger, s.refreshPolicy)
	dashHandler := handlers.NewDashboardHandler(s.cfg, s.dashboardStore, s.users, s.docsStore, s.incidentsStore, s.docsSvc, s.incidentsSvc, s.tasksStore, s.audits, s.policy, s.logger)
	placeholderHandler := handlers.NewPlaceholderHandler()
	settingsHandler := handlers.NewSettingsHandler()
	httpsSettingsHandler := handlers.NewHTTPSSettingsHandler(s.cfg, s.appHTTPSStore, s.audits)
	runtimeSettingsHandler := handlers.NewRuntimeSettingsHandler(s.cfg, s.appRuntimeStore, s.updateChecker, s.audits)
	docsHandler := handlers.NewDocsHandler(s.cfg, s.docsStore, s.entityLinksStore, s.controlsStore, s.users, s.policy, s.docsSvc, s.audits, s.logger)
	reportsHandler := handlers.NewReportsHandler(s.cfg, s.docsStore, s.reportsStore, s.users, s.policy, s.docsSvc, s.incidentsStore, s.incidentsSvc, s.controlsStore, s.monitoringStore, s.tasksSvc, s.audits, s.logger)
	incidentsHandler := handlers.NewIncidentsHandler(s.cfg, s.incidentsStore, s.entityLinksStore, s.controlsStore, s.users, s.docsStore, s.policy, s.incidentsSvc, s.docsSvc, s.audits, s.logger)
	controlsHandler := handlers.NewControlsHandler(s.controlsStore, s.entityLinksStore, s.users, s.docsStore, s.incidentsStore, s.tasksStore, s.audits, s.policy, s.logger)
	logsHandler := handlers.NewLogsHandler(s.audits)
	monitoringHandler := handlers.NewMonitoringHandler(s.monitoringStore, s.audits, s.monitoringEngine, s.policy, s.incidentsSvc.Encryptor())

	s.router.HandleFunc("/login", handlers.ServeStatic("login.html")).Methods("GET")
	s.router.HandleFunc("/password-change", s.withSession(authHandler.PasswordChangePage)).Methods("GET")
	s.router.HandleFunc("/", s.redirectToEntry)
	s.router.HandleFunc("/app", handlers.ServeStatic("app.html")).Methods("GET")
	appShell := s.withSession(s.requirePermission("app.view")(handlers.ServeStatic("app.html")))
	s.router.HandleFunc("/dashboard", appShell).Methods("GET")
	s.router.HandleFunc("/tasks", appShell).Methods("GET")
	s.router.HandleFunc("/tasks/task/{task_id:[0-9]+}", s.withSession(s.requirePermission("app.view")(s.redirectLegacyTaskLink))).Methods("GET")
	s.router.HandleFunc("/tasks/space/{space_id:[0-9]+}/task/{task_id:[0-9]+}", s.withSession(s.requirePermission("app.view")(s.taskSpaceTaskAppShell))).Methods("GET")
	s.router.HandleFunc("/tasks/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/docs", appShell).Methods("GET")
	s.router.HandleFunc("/docs/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/approvals", appShell).Methods("GET")
	s.router.HandleFunc("/approvals/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/incidents", appShell).Methods("GET")
	s.router.HandleFunc("/incidents/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/controls", appShell).Methods("GET")
	s.router.HandleFunc("/monitoring", appShell).Methods("GET")
	s.router.HandleFunc("/monitoring/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/reports", appShell).Methods("GET")
	s.router.HandleFunc("/findings", appShell).Methods("GET")
	s.router.HandleFunc("/accounts", appShell).Methods("GET")
	s.router.HandleFunc("/accounts/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/settings", appShell).Methods("GET")
	s.router.HandleFunc("/settings/{rest:.*}", appShell).Methods("GET")
	s.router.HandleFunc("/logs", appShell).Methods("GET")
	s.router.HandleFunc("/index.html", s.redirectToEntry)
	s.router.HandleFunc("/favicon.ico", handlers.ServeStatic("favicon.ico")).Methods("GET")

	apiRouter := s.router.PathPrefix("/api").Subrouter()
	apiRouter.Use(s.jsonMiddleware)

	apiRouter.HandleFunc("/auth/login", s.rateLimitMiddleware(authHandler.Login)).Methods("POST")
	apiRouter.HandleFunc("/auth/logout", s.withSession(authHandler.Logout)).Methods("POST")
	apiRouter.HandleFunc("/auth/me", s.withSession(authHandler.Me)).Methods("GET")
	apiRouter.HandleFunc("/auth/change-password", s.withSession(authHandler.ChangePassword)).Methods("POST")
	apiRouter.HandleFunc("/app/menu", s.withSession(authHandler.Menu)).Methods("GET")
	apiRouter.HandleFunc("/app/ping", s.withSession(authHandler.Ping)).Methods("POST")
	apiRouter.HandleFunc("/app/meta", s.withSession(s.requirePermission("app.view")(runtimeSettingsHandler.Meta))).Methods("GET")
	apiRouter.HandleFunc("/dashboard", s.withSession(s.requirePermission("dashboard.view")(dashHandler.Data))).Methods("GET")
	apiRouter.HandleFunc("/dashboard/layout", s.withSession(s.requirePermission("dashboard.view")(dashHandler.SaveLayout))).Methods("POST")

	accounts := apiRouter.PathPrefix("/accounts").Subrouter()
	accounts.HandleFunc("/dashboard", s.withSession(s.requirePermission("accounts.view_dashboard")(accHandler.Dashboard))).Methods("GET")
	accounts.HandleFunc("/users", s.withSession(s.requirePermission("accounts.view")(accHandler.ListUsers))).Methods("GET")
	accounts.HandleFunc("/users", s.withSession(s.requirePermission("accounts.manage")(accHandler.CreateUser))).Methods("POST")
	accounts.HandleFunc("/users/bulk", s.withSession(s.requirePermission("accounts.manage")(accHandler.BulkUsers))).Methods("POST")
	accounts.HandleFunc("/users/{id}", s.withSession(s.requirePermission("accounts.manage")(accHandler.UpdateUser))).Methods("PUT")
	accounts.HandleFunc("/users/{id}/reset-password", s.withSession(s.requirePermission("accounts.manage")(accHandler.ResetPassword))).Methods("POST")
	accounts.HandleFunc("/users/{id}/lock", s.withSession(s.requirePermission("accounts.manage")(accHandler.LockUser))).Methods("POST")
	accounts.HandleFunc("/users/{id}/unlock", s.withSession(s.requirePermission("accounts.manage")(accHandler.UnlockUser))).Methods("POST")
	accounts.HandleFunc("/users/{id}/sessions", s.withSession(s.requireAnyPermission("accounts.manage", "sessions.manage")(accHandler.ListSessions))).Methods("GET")
	accounts.HandleFunc("/users/{id}/sessions/kill_all", s.withSession(s.requireAnyPermission("accounts.manage", "sessions.manage")(accHandler.KillAllSessions))).Methods("POST")
	accounts.HandleFunc("/sessions/{session_id}/kill", s.withSession(s.requireAnyPermission("accounts.manage", "sessions.manage")(accHandler.KillSession))).Methods("POST")
	accounts.HandleFunc("/groups", s.withSession(s.requirePermission("groups.view")(accHandler.ListGroups))).Methods("GET")
	accounts.HandleFunc("/groups", s.withSession(s.requirePermission("groups.manage")(accHandler.CreateGroup))).Methods("POST")
	accounts.HandleFunc("/groups/{id}", s.withSession(s.requirePermission("groups.view")(accHandler.GetGroup))).Methods("GET")
	accounts.HandleFunc("/groups/{id}", s.withSession(s.requirePermission("groups.manage")(accHandler.UpdateGroup))).Methods("PUT")
	accounts.HandleFunc("/groups/{id}", s.withSession(s.requirePermission("groups.manage")(accHandler.DeleteGroup))).Methods("DELETE")
	accounts.HandleFunc("/groups/{id}/members", s.withSession(s.requirePermission("groups.manage")(accHandler.AddGroupMember))).Methods("POST")
	accounts.HandleFunc("/groups/{id}/members/{user_id}", s.withSession(s.requirePermission("groups.manage")(accHandler.RemoveGroupMember))).Methods("DELETE")
	accounts.HandleFunc("/users/{id}/groups", s.withSession(s.requirePermission("groups.manage")(accHandler.ListUserGroups))).Methods("GET")
	accounts.HandleFunc("/roles", s.withSession(s.requirePermission("roles.view")(accHandler.ListRoles))).Methods("GET")
	accounts.HandleFunc("/roles", s.withSession(s.requirePermission("roles.manage")(accHandler.CreateRole))).Methods("POST")
	accounts.HandleFunc("/roles/{id}", s.withSession(s.requirePermission("roles.manage")(accHandler.UpdateRole))).Methods("PUT")
	accounts.HandleFunc("/roles/{id}", s.withSession(s.requirePermission("roles.manage")(accHandler.DeleteRole))).Methods("DELETE")
	accounts.HandleFunc("/role-templates", s.withSession(s.requirePermission("roles.view")(accHandler.ListRoleTemplates))).Methods("GET")
	accounts.HandleFunc("/roles/from-template", s.withSession(s.requirePermission("roles.manage")(accHandler.CreateRoleFromTemplate))).Methods("POST")
	accounts.HandleFunc("/import/upload", s.withSession(s.requirePermission("accounts.manage")(accHandler.ImportUpload))).Methods("POST")
	accounts.HandleFunc("/import/commit", s.withSession(s.requirePermission("accounts.manage")(accHandler.ImportCommit))).Methods("POST")
	accounts.HandleFunc("/import", s.withSession(s.requirePermission("accounts.manage")(accHandler.ImportUsers))).Methods("POST")

	foldersRouter := apiRouter.PathPrefix("/docs/folders").Subrouter()
	foldersRouter.HandleFunc("", s.withSession(s.requirePermission("folders.view")(docsHandler.ListFolders))).Methods("GET")
	foldersRouter.HandleFunc("", s.withSession(s.requirePermission("folders.manage")(docsHandler.CreateFolder))).Methods("POST")
	foldersRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("folders.manage")(docsHandler.UpdateFolder))).Methods("PUT")
	foldersRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("folders.manage")(docsHandler.DeleteFolder))).Methods("DELETE")

	docsRouter := apiRouter.PathPrefix("/docs").Subrouter()
	docsRouter.HandleFunc("", s.withSession(s.requirePermission("docs.view")(docsHandler.List))).Methods("GET")
	docsRouter.HandleFunc("", s.withSession(s.requirePermission("docs.create")(docsHandler.Create))).Methods("POST")
	docsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("docs.view")(docsHandler.Get))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("docs.delete")(docsHandler.DeleteDocument))).Methods("DELETE")
	docsRouter.HandleFunc("/{id:[0-9]+}/content", s.withSession(s.requirePermission("docs.view")(docsHandler.GetContent))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/content", s.withSession(s.requirePermission("docs.edit")(docsHandler.UpdateContent))).Methods("PUT")
	docsRouter.HandleFunc("/{id:[0-9]+}/versions", s.withSession(s.requirePermission("docs.versions.view")(docsHandler.ListVersions))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/versions/{ver:[0-9]+}/content", s.withSession(s.requirePermission("docs.versions.view")(docsHandler.GetVersionContent))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/versions/{ver:[0-9]+}/restore", s.withSession(s.requirePermission("docs.versions.restore")(docsHandler.RestoreVersion))).Methods("POST")
	docsRouter.HandleFunc("/{id:[0-9]+}/export", s.withSession(s.requirePermission("docs.export")(docsHandler.Export))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/acl", s.withSession(s.requirePermission("docs.manage")(docsHandler.GetACL))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/acl", s.withSession(s.requirePermission("docs.manage")(docsHandler.UpdateACL))).Methods("PUT")
	docsRouter.HandleFunc("/{id:[0-9]+}/classification", s.withSession(s.requirePermission("docs.classification.set")(docsHandler.UpdateClassification))).Methods("POST")
	docsRouter.HandleFunc("/{id:[0-9]+}/approval/start", s.withSession(s.requirePermission("docs.approval.start")(docsHandler.StartApproval))).Methods("POST")
	docsRouter.HandleFunc("/{id:[0-9]+}/links", s.withSession(s.requirePermission("docs.view")(docsHandler.ListLinks))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/links", s.withSession(s.requirePermission("docs.edit")(docsHandler.AddLink))).Methods("POST")
	docsRouter.HandleFunc("/{id:[0-9]+}/links/{link_id:[0-9]+}", s.withSession(s.requirePermission("docs.edit")(docsHandler.DeleteLink))).Methods("DELETE")
	docsRouter.HandleFunc("/{id:[0-9]+}/control-links", s.withSession(s.requirePermission("docs.view")(docsHandler.ListControlLinks))).Methods("GET")
	docsRouter.HandleFunc("/{id:[0-9]+}/convert", s.withSession(s.requirePermission("docs.edit")(docsHandler.ConvertToMarkdown))).Methods("POST")
	docsRouter.HandleFunc("/upload", s.withSession(s.requirePermission("docs.upload")(docsHandler.Upload))).Methods("POST")
	docsRouter.HandleFunc("/import/commit", s.withSession(s.requirePermission("docs.upload")(docsHandler.ImportCommit))).Methods("POST")
	docsRouter.HandleFunc("/list", s.withSession(s.requirePermission("docs.view")(incidentsHandler.ListDocsLite))).Methods("GET")

	reportsRouter := apiRouter.PathPrefix("/reports").Subrouter()
	reportsRouter.HandleFunc("", s.withSession(s.requirePermission("reports.view")(reportsHandler.List))).Methods("GET")
	reportsRouter.HandleFunc("", s.withSession(s.requirePermission("reports.create")(reportsHandler.Create))).Methods("POST")
	reportsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("reports.view")(reportsHandler.Get))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("reports.edit")(reportsHandler.UpdateMeta))).Methods("PUT")
	reportsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("reports.delete")(reportsHandler.Delete))).Methods("DELETE")
	reportsRouter.HandleFunc("/{id:[0-9]+}/content", s.withSession(s.requirePermission("reports.view")(reportsHandler.GetContent))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/content", s.withSession(s.requirePermission("reports.edit")(reportsHandler.UpdateContent))).Methods("PUT")
	reportsRouter.HandleFunc("/{id:[0-9]+}/export", s.withSession(s.requirePermission("reports.export")(reportsHandler.Export))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/sections", s.withSession(s.requirePermission("reports.view")(reportsHandler.ListSections))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/sections", s.withSession(s.requirePermission("reports.edit")(reportsHandler.UpdateSections))).Methods("PUT")
	reportsRouter.HandleFunc("/{id:[0-9]+}/charts", s.withSession(s.requirePermission("reports.view")(reportsHandler.ListCharts))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/charts", s.withSession(s.requirePermission("reports.edit")(reportsHandler.UpdateCharts))).Methods("PUT")
	reportsRouter.HandleFunc("/{id:[0-9]+}/charts/{chart_id:[0-9]+}/render", s.withSession(s.requirePermission("reports.view")(reportsHandler.RenderChart))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/build", s.withSession(s.requirePermission("reports.edit")(reportsHandler.Build))).Methods("POST")
	reportsRouter.HandleFunc("/{id:[0-9]+}/snapshots", s.withSession(s.requirePermission("reports.view")(reportsHandler.ListSnapshots))).Methods("GET")
	reportsRouter.HandleFunc("/{id:[0-9]+}/snapshots/{snapshot_id:[0-9]+}", s.withSession(s.requirePermission("reports.view")(reportsHandler.GetSnapshot))).Methods("GET")
	reportsRouter.HandleFunc("/templates", s.withSession(s.requirePermission("reports.templates.view")(reportsHandler.ListTemplates))).Methods("GET")
	reportsRouter.HandleFunc("/templates", s.withSession(s.requirePermission("reports.templates.manage")(reportsHandler.SaveTemplate))).Methods("POST")
	reportsRouter.HandleFunc("/templates/{id:[0-9]+}", s.withSession(s.requirePermission("reports.templates.manage")(reportsHandler.SaveTemplate))).Methods("PUT")
	reportsRouter.HandleFunc("/templates/{id:[0-9]+}", s.withSession(s.requirePermission("reports.templates.manage")(reportsHandler.DeleteTemplate))).Methods("DELETE")
	reportsRouter.HandleFunc("/from-template", s.withSession(s.requirePermission("reports.create")(reportsHandler.CreateFromTemplate))).Methods("POST")
	reportsRouter.HandleFunc("/from-incident", s.withSession(s.requirePermission("reports.create")(reportsHandler.CreateFromIncident))).Methods("POST")
	reportsRouter.HandleFunc("/settings", s.withSession(s.requirePermission("reports.templates.manage")(reportsHandler.GetSettings))).Methods("GET")
	reportsRouter.HandleFunc("/settings", s.withSession(s.requirePermission("reports.templates.manage")(reportsHandler.UpdateSettings))).Methods("PUT")

	incidentsRouter := apiRouter.PathPrefix("/incidents").Subrouter()
	incidentsRouter.HandleFunc("/dashboard", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.Dashboard))).Methods("GET")
	incidentsRouter.HandleFunc("/list", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListIncidentsLite))).Methods("GET")
	incidentsRouter.HandleFunc("", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.List))).Methods("GET")
	incidentsRouter.HandleFunc("", s.withSession(s.requirePermission("incidents.create")(incidentsHandler.Create))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.Get))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.Update))).Methods("PUT")
	incidentsRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("incidents.delete")(incidentsHandler.Delete))).Methods("DELETE")
	incidentsRouter.HandleFunc("/{id}/restore", s.withSession(s.requirePermission("incidents.manage")(incidentsHandler.Restore))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/links", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListLinks))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/links", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.AddLink))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/links/{link_id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.DeleteLink))).Methods("DELETE")
	incidentsRouter.HandleFunc("/{id}/control-links", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListControlLinks))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/attachments", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListAttachments))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/attachments/upload", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.UploadAttachment))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/attachments/{att_id}/download", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.DownloadAttachment))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/attachments/{att_id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.DeleteAttachment))).Methods("DELETE")
	incidentsRouter.HandleFunc("/{id}/artifacts/{artifact_id}/files", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListArtifactFiles))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/artifacts/{artifact_id}/files", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.UploadArtifactFile))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/artifacts/{artifact_id}/files/{file_id}/download", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.DownloadArtifactFile))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/artifacts/{artifact_id}/files/{file_id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.DeleteArtifactFile))).Methods("DELETE")
	incidentsRouter.HandleFunc("/{id}/timeline", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListTimeline))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/timeline", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.AddTimeline))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/activity", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListActivity))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/export", s.withSession(s.requirePermission("incidents.export")(incidentsHandler.Export))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/create-report-doc", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.CreateReportDoc))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/close", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.CloseIncident))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/stages", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.ListStages))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/stages", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.AddStage))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.GetStage))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.UpdateStage))).Methods("PUT")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.DeleteStage))).Methods("DELETE")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}/complete", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.CompleteStage))).Methods("POST")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}/content", s.withSession(s.requirePermission("incidents.view")(incidentsHandler.GetStageContent))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/stages/{stage_id}/content", s.withSession(s.requirePermission("incidents.edit")(incidentsHandler.UpdateStageContent))).Methods("PUT")
	incidentsRouter.HandleFunc("/{id}/acl", s.withSession(s.requirePermission("incidents.manage")(incidentsHandler.GetACL))).Methods("GET")
	incidentsRouter.HandleFunc("/{id}/acl", s.withSession(s.requirePermission("incidents.manage")(incidentsHandler.UpdateACL))).Methods("PUT")

	controlsRouter := apiRouter.PathPrefix("/controls").Subrouter()
	controlsRouter.HandleFunc("", s.withSession(s.requirePermission("controls.view")(controlsHandler.ListControls))).Methods("GET")
	controlsRouter.HandleFunc("", s.withSession(s.requirePermission("controls.manage")(controlsHandler.CreateControl))).Methods("POST")
	controlsRouter.HandleFunc("/types", s.withSession(s.requirePermission("controls.view")(controlsHandler.ListControlTypes))).Methods("GET")
	controlsRouter.HandleFunc("/types", s.withSession(s.requirePermission("settings.controls")(controlsHandler.CreateControlType))).Methods("POST")
	controlsRouter.HandleFunc("/types/{id:[0-9]+}", s.withSession(s.requirePermission("settings.controls")(controlsHandler.DeleteControlType))).Methods("DELETE")
	controlsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("controls.view")(controlsHandler.GetControl))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("controls.manage")(controlsHandler.UpdateControl))).Methods("PUT")
	controlsRouter.HandleFunc("/{id:[0-9]+}", s.withSession(s.requirePermission("controls.manage")(controlsHandler.DeleteControl))).Methods("DELETE")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments", s.withSession(s.requirePermission("controls.view")(controlsHandler.ListControlComments))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments", s.withSession(s.requirePermission("controls.manage")(controlsHandler.AddControlComment))).Methods("POST")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments/{comment_id:[0-9]+}", s.withSession(s.requirePermission("controls.view")(controlsHandler.UpdateControlComment))).Methods("PUT")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments/{comment_id:[0-9]+}", s.withSession(s.requirePermission("controls.view")(controlsHandler.DeleteControlComment))).Methods("DELETE")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments/{comment_id:[0-9]+}/files/{file_id}", s.withSession(s.requirePermission("controls.view")(controlsHandler.DownloadControlCommentFile))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}/comments/{comment_id:[0-9]+}/files/{file_id}", s.withSession(s.requirePermission("controls.view")(controlsHandler.DeleteControlCommentFile))).Methods("DELETE")
	controlsRouter.HandleFunc("/{id:[0-9]+}/checks", s.withSession(s.requirePermission("controls.checks.view")(controlsHandler.ListControlChecks))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}/checks", s.withSession(s.requirePermission("controls.checks.manage")(controlsHandler.CreateControlCheck))).Methods("POST")
	controlsRouter.HandleFunc("/{id:[0-9]+}/violations", s.withSession(s.requirePermission("controls.violations.view")(controlsHandler.ListControlViolations))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}/violations", s.withSession(s.requirePermission("controls.violations.manage")(controlsHandler.CreateControlViolation))).Methods("POST")
	controlsRouter.HandleFunc("/{id:[0-9]+}/links", s.withSession(s.requirePermission("controls.view")(controlsHandler.ListControlLinks))).Methods("GET")
	controlsRouter.HandleFunc("/{id:[0-9]+}/links", s.withSession(s.requirePermission("controls.manage")(controlsHandler.CreateControlLink))).Methods("POST")
	controlsRouter.HandleFunc("/{id:[0-9]+}/links/{link_id:[0-9]+}", s.withSession(s.requirePermission("controls.manage")(controlsHandler.DeleteControlLink))).Methods("DELETE")

	apiRouter.HandleFunc("/checks", s.withSession(s.requirePermission("controls.checks.view")(controlsHandler.ListChecks))).Methods("GET")
	apiRouter.HandleFunc("/checks/{id:[0-9]+}", s.withSession(s.requirePermission("controls.checks.manage")(controlsHandler.DeleteCheck))).Methods("DELETE")
	apiRouter.HandleFunc("/violations", s.withSession(s.requirePermission("controls.violations.view")(controlsHandler.ListViolations))).Methods("GET")
	apiRouter.HandleFunc("/violations/{id:[0-9]+}", s.withSession(s.requirePermission("controls.violations.manage")(controlsHandler.DeleteViolation))).Methods("DELETE")

	frameworksRouter := apiRouter.PathPrefix("/frameworks").Subrouter()
	frameworksRouter.HandleFunc("", s.withSession(s.requirePermission("controls.frameworks.view")(controlsHandler.ListFrameworks))).Methods("GET")
	frameworksRouter.HandleFunc("", s.withSession(s.requirePermission("controls.frameworks.manage")(controlsHandler.CreateFramework))).Methods("POST")
	frameworksRouter.HandleFunc("/{id:[0-9]+}/items", s.withSession(s.requirePermission("controls.frameworks.view")(controlsHandler.ListFrameworkItems))).Methods("GET")
	frameworksRouter.HandleFunc("/{id:[0-9]+}/items", s.withSession(s.requirePermission("controls.frameworks.manage")(controlsHandler.CreateFrameworkItem))).Methods("POST")
	frameworksRouter.HandleFunc("/map", s.withSession(s.requirePermission("controls.frameworks.manage")(controlsHandler.CreateFrameworkMap))).Methods("POST")
	frameworksRouter.HandleFunc("/{id:[0-9]+}/map", s.withSession(s.requirePermission("controls.frameworks.view")(controlsHandler.ListFrameworkMap))).Methods("GET")

	monitoringRouter := apiRouter.PathPrefix("/monitoring").Subrouter()
	monitoringRouter.HandleFunc("/monitors", s.withSession(s.requirePermission("monitoring.view")(monitoringHandler.ListMonitors))).Methods("GET")
	monitoringRouter.HandleFunc("/monitors", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.CreateMonitor))).Methods("POST")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.view")(monitoringHandler.GetMonitor))).Methods("GET")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.UpdateMonitor))).Methods("PUT")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.DeleteMonitor))).Methods("DELETE")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/pause", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.PauseMonitor))).Methods("POST")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/resume", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.ResumeMonitor))).Methods("POST")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/check-now", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.CheckNow))).Methods("POST")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/clone", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.CloneMonitor))).Methods("POST")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/state", s.withSession(s.requirePermission("monitoring.view")(monitoringHandler.GetState))).Methods("GET")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/metrics", s.withSession(s.requirePermission("monitoring.view")(monitoringHandler.GetMetrics))).Methods("GET")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/metrics", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.DeleteMonitorMetrics))).Methods("DELETE")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/events", s.withSession(s.requirePermission("monitoring.events.view")(monitoringHandler.GetEvents))).Methods("GET")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/events", s.withSession(s.requirePermission("monitoring.manage")(monitoringHandler.DeleteMonitorEvents))).Methods("DELETE")
		monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/tls", s.withSession(s.requirePermission("monitoring.certs.view")(monitoringHandler.GetTLS))).Methods("GET")
	monitoringRouter.HandleFunc("/certs", s.withSession(s.requirePermission("monitoring.certs.view")(monitoringHandler.ListCerts))).Methods("GET")
	monitoringRouter.HandleFunc("/certs/test-notification", s.withSession(s.requirePermission("monitoring.certs.manage")(monitoringHandler.TestCertNotification))).Methods("POST")
	monitoringRouter.HandleFunc("/events", s.withSession(s.requirePermission("monitoring.events.view")(monitoringHandler.EventsFeed))).Methods("GET")
	monitoringRouter.HandleFunc("/maintenance", s.withSession(s.requirePermission("monitoring.maintenance.view")(monitoringHandler.ListMaintenance))).Methods("GET")
	monitoringRouter.HandleFunc("/maintenance", s.withSession(s.requirePermission("monitoring.maintenance.manage")(monitoringHandler.CreateMaintenance))).Methods("POST")
	monitoringRouter.HandleFunc("/maintenance/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.maintenance.manage")(monitoringHandler.UpdateMaintenance))).Methods("PUT")
	monitoringRouter.HandleFunc("/maintenance/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.maintenance.manage")(monitoringHandler.DeleteMaintenance))).Methods("DELETE")
	monitoringRouter.HandleFunc("/settings", s.withSession(s.requirePermission("monitoring.settings.manage")(monitoringHandler.GetSettings))).Methods("GET")
	monitoringRouter.HandleFunc("/settings", s.withSession(s.requirePermission("monitoring.settings.manage")(monitoringHandler.UpdateSettings))).Methods("PUT")
	monitoringRouter.HandleFunc("/notifications", s.withSession(s.requirePermission("monitoring.notifications.view")(monitoringHandler.ListNotificationChannels))).Methods("GET")
	monitoringRouter.HandleFunc("/notifications", s.withSession(s.requirePermission("monitoring.notifications.manage")(monitoringHandler.CreateNotificationChannel))).Methods("POST")
	monitoringRouter.HandleFunc("/notifications/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.notifications.manage")(monitoringHandler.UpdateNotificationChannel))).Methods("PUT")
	monitoringRouter.HandleFunc("/notifications/{id:[0-9]+}", s.withSession(s.requirePermission("monitoring.notifications.manage")(monitoringHandler.DeleteNotificationChannel))).Methods("DELETE")
	monitoringRouter.HandleFunc("/notifications/{id:[0-9]+}/test", s.withSession(s.requirePermission("monitoring.notifications.manage")(monitoringHandler.TestNotificationChannel))).Methods("POST")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/notifications", s.withSession(s.requirePermission("monitoring.notifications.view")(monitoringHandler.ListMonitorNotifications))).Methods("GET")
	monitoringRouter.HandleFunc("/monitors/{id:[0-9]+}/notifications", s.withSession(s.requirePermission("monitoring.notifications.manage")(monitoringHandler.UpdateMonitorNotifications))).Methods("PUT")

	taskHandler := taskhttp.NewHandler(s.cfg, s.tasksSvc, s.users, s.docsStore, s.docsSvc, s.incidentsStore, s.incidentsSvc, s.controlsStore, s.entityLinksStore, s.policy, s.audits)
	taskhttp.RegisterRoutes(taskhttp.RouteDeps{
		Router:            apiRouter,
		WithSession:       s.withSession,
		RequirePermission: s.requirePermission,
		Handler:           taskHandler,
	})

	templatesRouter := apiRouter.PathPrefix("/templates").Subrouter()
	templatesRouter.HandleFunc("", s.withSession(s.requirePermission("templates.view")(docsHandler.ListTemplates))).Methods("GET")
	templatesRouter.HandleFunc("", s.withSession(s.requirePermission("templates.manage")(docsHandler.SaveTemplate))).Methods("POST")
	templatesRouter.HandleFunc("/{id}", s.withSession(s.requirePermission("templates.manage")(docsHandler.DeleteTemplate))).Methods("DELETE")

	approvalsRouter := apiRouter.PathPrefix("/approvals").Subrouter()
	approvalsRouter.HandleFunc("/cleanup", s.withSession(s.requirePermission("docs.approval.view")(docsHandler.CleanupApprovals))).Methods("POST")
	approvalsRouter.HandleFunc("", s.withSession(s.requirePermission("docs.approval.view")(docsHandler.ListApprovals))).Methods("GET")
	approvalsRouter.HandleFunc("/{approval_id}", s.withSession(s.requirePermission("docs.approval.view")(docsHandler.GetApproval))).Methods("GET")
	approvalsRouter.HandleFunc("/{approval_id}/decision", s.withSession(s.requirePermission("docs.approval.approve")(docsHandler.ApprovalDecision))).Methods("POST")
	approvalsRouter.HandleFunc("/{approval_id}/comments", s.withSession(s.requirePermission("docs.approval.view")(docsHandler.ListApprovalComments))).Methods("GET")
	approvalsRouter.HandleFunc("/{approval_id}/comments", s.withSession(s.requirePermission("docs.approval.view")(docsHandler.AddApprovalComment))).Methods("POST")

	logsRouter := apiRouter.PathPrefix("/logs").Subrouter()
	logsRouter.HandleFunc("", s.withSession(s.requirePermission("logs.view")(logsHandler.List))).Methods("GET")
	apiRouter.HandleFunc("/settings/https", s.withSession(s.requirePermission("settings.advanced")(httpsSettingsHandler.Get))).Methods("GET")
	apiRouter.HandleFunc("/settings/https", s.withSession(s.requirePermission("settings.advanced")(httpsSettingsHandler.Update))).Methods("PUT")
	apiRouter.HandleFunc("/settings/runtime", s.withSession(s.requirePermission("settings.advanced")(runtimeSettingsHandler.Get))).Methods("GET")
	apiRouter.HandleFunc("/settings/runtime", s.withSession(s.requirePermission("settings.advanced")(runtimeSettingsHandler.Update))).Methods("PUT")
	apiRouter.HandleFunc("/settings/updates/check", s.withSession(s.requirePermission("settings.advanced")(runtimeSettingsHandler.CheckUpdates))).Methods("POST")

	pageRouter := s.router.PathPrefix("/api/page").Subrouter()
	pageRouter.HandleFunc("/dashboard", s.withSession(s.requirePermission("dashboard.view")(dashHandler.Dashboard))).Methods("GET")
	pageRouter.HandleFunc("/settings", s.withSession(s.requirePermission("app.view")(settingsHandler.Page))).Methods("GET")
	pageRouter.HandleFunc("/accounts", s.withSession(s.requirePermission("accounts.view")(accHandler.Page))).Methods("GET")
	pageRouter.HandleFunc("/{name}", s.withSession(s.requirePermissionFromPath(handlers.RequiredPermission)(placeholderHandler.Page))).Methods("GET")
}

func (s *Server) redirectToEntry(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	sr, err := s.sessions.GetSession(r.Context(), cookie.Value)
	if err != nil || sr == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	user, _, err := s.users.FindByUsername(r.Context(), sr.Username)
	if err != nil || user == nil || !user.Active {
		_ = s.sessions.DeleteSession(r.Context(), sr.ID, sr.Username)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if user.RequirePasswordChange {
		http.Redirect(w, r, "/password-change", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) staticHandler() http.Handler {
	staticFS, err := fs.Sub(gui.StaticFiles, "static")
	if err != nil {
		s.logger.Fatalf("static fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := mime.TypeByExtension(filepath.Ext(r.URL.Path)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		fileServer.ServeHTTP(w, r)
	})
}

func ensureMimeTypes() {
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".json", "application/json; charset=utf-8")
	_ = mime.AddExtensionType(".html", "text/html; charset=utf-8")
}

func (s *Server) bootstrapRoles(ctx context.Context) error {
	if err := s.roles.EnsureBuiltIn(ctx, defaultStoreRoles()); err != nil {
		return err
	}
	return s.refreshPolicy(ctx)
}

func (s *Server) refreshPolicy(ctx context.Context) error {
	roles, err := s.roles.List(ctx)
	if err != nil {
		return err
	}
	var rbacRoles []rbac.Role
	for _, r := range roles {
		perms := make([]rbac.Permission, 0, len(r.Permissions))
		for _, p := range r.Permissions {
			perms = append(perms, rbac.Permission(p))
		}
		rbacRoles = append(rbacRoles, rbac.Role{Name: r.Name, Permissions: perms})
	}
	s.policy.Replace(rbacRoles)
	return nil
}

func defaultStoreRoles() []store.Role {
	def := rbac.DefaultRoles()
	res := make([]store.Role, 0, len(def))
	for _, r := range def {
		perms := make([]string, 0, len(r.Permissions))
		for _, p := range r.Permissions {
			perms = append(perms, string(p))
		}
		res = append(res, store.Role{
			Name:        r.Name,
			Description: strings.ReplaceAll(r.Name, "_", " "),
			Permissions: perms,
			BuiltIn:     true,
		})
	}
	return res
}
