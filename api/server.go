package api

import (
	"context"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/appmeta"
	"berkut-scc/core/auth"
	"berkut-scc/core/backups"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/gui"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	cfg              *config.AppConfig
	router           *chi.Mux
	httpServer       *http.Server
	logger           *utils.Logger
	sessionManager   *auth.SessionManager
	users            store.UsersStore
	sessions         store.SessionStore
	roles            store.RolesStore
	groups           store.GroupsStore
	audits           store.AuditStore
	policy           *rbac.Policy
	docsStore        store.DocsStore
	reportsStore     store.ReportsStore
	docsSvc          *docs.Service
	incidentsStore   store.IncidentsStore
	incidentsSvc     *incidents.Service
	tasksStore       tasks.Store
	tasksSvc         *tasks.Service
	dashboardStore   store.DashboardStore
	backupsSvc       *backups.Service
	controlsStore    store.ControlsStore
	entityLinksStore store.EntityLinksStore
	monitoringStore  store.MonitoringStore
	appHTTPSStore    store.AppHTTPSStore
	appRuntimeStore  store.AppRuntimeStore
	updateChecker    *appmeta.UpdateChecker
	monitoringEngine *monitoring.Engine
	activityTracker  *sessionActivity
}

func NewServer(cfg *config.AppConfig, logger *utils.Logger, deps ServerDeps) *Server {
	ensureMimeTypes()
	s := &Server{
		cfg:              cfg,
		router:           chi.NewRouter(),
		logger:           logger,
		sessionManager:   auth.NewSessionManager(deps.Sessions, cfg, logger),
		users:            deps.Users,
		sessions:         deps.Sessions,
		roles:            deps.Roles,
		groups:           deps.Groups,
		audits:           deps.Audits,
		policy:           rbac.NewPolicy(nil),
		docsStore:        deps.DocsStore,
		reportsStore:     deps.ReportsStore,
		docsSvc:          deps.DocsSvc,
		incidentsStore:   deps.IncidentsStore,
		incidentsSvc:     deps.IncidentsSvc,
		controlsStore:    deps.ControlsStore,
		entityLinksStore: deps.EntityLinksStore,
		monitoringStore:  deps.MonitoringStore,
		appHTTPSStore:    deps.AppHTTPSStore,
		appRuntimeStore:  deps.AppRuntimeStore,
		updateChecker:    deps.UpdateChecker,
		monitoringEngine: deps.MonitoringEngine,
		tasksStore:       deps.TasksStore,
		tasksSvc:         deps.TasksSvc,
		dashboardStore:   deps.DashboardStore,
		backupsSvc:       deps.BackupsSvc,
		activityTracker:  newSessionActivity(),
	}
	if err := s.bootstrapRoles(context.Background()); err != nil && logger != nil {
		logger.Errorf("bootstrap roles: %v", err)
	}
	s.registerRoutes()
	return s
}

func (s *Server) Start() error {
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
	return s.httpServer.Shutdown(ctx)
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
	return rbac.EnsureBuiltInAndRefresh(ctx, s.roles, s.policy)
}

func (s *Server) refreshPolicy(ctx context.Context) error {
	return rbac.RefreshFromStore(ctx, s.roles, s.policy)
}
