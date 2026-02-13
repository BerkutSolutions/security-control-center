package rbac

import (
	"context"
	"strings"

	"berkut-scc/core/store"
)

func EnsureBuiltInAndRefresh(ctx context.Context, roles store.RolesStore, policy *Policy) error {
	if roles == nil || policy == nil {
		return nil
	}
	if err := roles.EnsureBuiltIn(ctx, defaultStoreRoles()); err != nil {
		return err
	}
	return RefreshFromStore(ctx, roles, policy)
}

func RefreshFromStore(ctx context.Context, roles store.RolesStore, policy *Policy) error {
	if roles == nil || policy == nil {
		return nil
	}
	items, err := roles.List(ctx)
	if err != nil {
		return err
	}
	out := make([]Role, 0, len(items))
	for _, item := range items {
		perms := make([]Permission, 0, len(item.Permissions))
		for _, p := range item.Permissions {
			perms = append(perms, Permission(p))
		}
		out = append(out, Role{Name: item.Name, Permissions: perms})
	}
	policy.Replace(out)
	return nil
}

func defaultStoreRoles() []store.Role {
	def := DefaultRoles()
	out := make([]store.Role, 0, len(def))
	for _, r := range def {
		perms := make([]string, 0, len(r.Permissions))
		for _, p := range r.Permissions {
			perms = append(perms, string(p))
		}
		out = append(out, store.Role{
			Name:        r.Name,
			Description: strings.ReplaceAll(r.Name, "_", " "),
			Permissions: perms,
			BuiltIn:     true,
		})
	}
	return out
}
