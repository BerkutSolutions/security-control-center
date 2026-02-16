package routegroups

import "net/http"

type Guards struct {
	WithSession          func(http.HandlerFunc) http.HandlerFunc
	RequirePermission    func(string) func(http.HandlerFunc) http.HandlerFunc
	RequireAnyPermission func(...string) func(http.HandlerFunc) http.HandlerFunc
}

func (g Guards) Session(handler http.HandlerFunc) http.HandlerFunc {
	return g.WithSession(handler)
}

func (g Guards) SessionPerm(perm string, handler http.HandlerFunc) http.HandlerFunc {
	return g.WithSession(g.RequirePermission(perm)(handler))
}

func (g Guards) SessionAnyPerm(perms []string, handler http.HandlerFunc) http.HandlerFunc {
	return g.WithSession(g.RequireAnyPermission(perms...)(handler))
}
