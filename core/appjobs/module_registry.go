package appjobs

import (
	"context"
	"strings"

	"berkut-scc/core/appcompat"
)

type Module interface {
	ID() string
	HasFullReset() bool
	ExpectedSchemaVersion() int
	ExpectedBehaviorVersion() int

	FullReset(ctx context.Context, deps ModuleDeps) (ModuleResult, error)
	PartialAdapt(ctx context.Context, deps ModuleDeps) (ModuleResult, error)
}

type moduleSpec struct {
	id              string
	hasFullReset    bool
	expectedSchema  int
	expectedBehavior int

	full    func(context.Context, ModuleDeps) (ModuleResult, error)
	partial func(context.Context, ModuleDeps) (ModuleResult, error)
}

func (m moduleSpec) ID() string                     { return m.id }
func (m moduleSpec) HasFullReset() bool             { return m.hasFullReset }
func (m moduleSpec) ExpectedSchemaVersion() int     { return m.expectedSchema }
func (m moduleSpec) ExpectedBehaviorVersion() int   { return m.expectedBehavior }
func (m moduleSpec) FullReset(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
	if m.full == nil {
		return ModuleResult{}, nil
	}
	return m.full(ctx, deps)
}
func (m moduleSpec) PartialAdapt(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
	if m.partial == nil {
		return ModuleResult{}, nil
	}
	return m.partial(ctx, deps)
}

type Registry struct {
	modules map[string]Module
	order   []string
}

func DefaultModuleRegistry() *Registry {
	reg := &Registry{modules: map[string]Module{}}
	// Keep module IDs in sync with core/appcompat/registry.go.
	for _, spec := range appcompat.DefaultRegistry() {
		id := strings.TrimSpace(spec.ModuleID)
		if id == "" {
			continue
		}
		// Each module is implemented in its own file.
		switch id {
		case "dashboard":
			reg.modules[id] = moduleDashboard(spec)
		case "tasks":
			reg.modules[id] = moduleTasks(spec)
		case "monitoring":
			reg.modules[id] = moduleMonitoring(spec)
		case "docs":
			reg.modules[id] = moduleDocs(spec)
		case "approvals":
			reg.modules[id] = moduleApprovals(spec)
		case "incidents":
			reg.modules[id] = moduleIncidents(spec)
		case "registry.controls":
			reg.modules[id] = moduleRegistryControls(spec)
		case "registry.assets":
			reg.modules[id] = moduleRegistryAssets(spec)
		case "registry.software":
			reg.modules[id] = moduleRegistrySoftware(spec)
		case "registry.findings":
			reg.modules[id] = moduleRegistryFindings(spec)
		case "reports":
			reg.modules[id] = moduleReports(spec)
		case "accounts":
			reg.modules[id] = moduleAccounts(spec)
		case "settings":
			reg.modules[id] = moduleSettings(spec)
		case "backups":
			reg.modules[id] = moduleBackups(spec)
		case "logs":
			reg.modules[id] = moduleLogs(spec)
		}
		if reg.modules[id] != nil {
			reg.order = append(reg.order, id)
		}
	}
	return reg
}

func (r *Registry) Get(moduleID string) Module {
	if r == nil {
		return nil
	}
	return r.modules[strings.TrimSpace(moduleID)]
}

func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.order))
	out = append(out, r.order...)
	return out
}
