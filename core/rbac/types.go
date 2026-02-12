package rbac

type Permission string

type Role struct {
	Name        string
	Permissions []Permission
}
