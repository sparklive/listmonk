package core

import (
	"encoding/json"
	"net/http"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetRoles retrieves all roles.
func (c *Core) GetRoles() ([]models.Role, error) {
	out := []models.Role{}
	if err := c.q.GetUserRoles.Select(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "role", "error", pqErrMsg(err)))
	}

	return out, nil
}

// GetListRoles retrieves all list roles.
func (c *Core) GetListRoles() ([]models.ListRole, error) {
	out := []models.ListRole{}
	if err := c.q.GetListRoles.Select(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "role", "error", pqErrMsg(err)))
	}

	// Unmarshall the nested list permissions, if any.
	for n, r := range out {
		if r.ListsRaw == nil {
			continue
		}

		if err := json.Unmarshal(r.ListsRaw, &out[n].Lists); err != nil {
			c.log.Printf("error unmarshalling list permissions for role %d: %v", r.ID, err)
		}
	}

	return out, nil
}

// CreateRole creates a new role.
func (c *Core) CreateRole(r models.Role) (models.Role, error) {
	var out models.Role

	if err := c.q.CreateRole.Get(&out, r.Name, models.RoleTypeUser, pq.Array(r.Permissions)); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// CreateListRole creates a new list role.
func (c *Core) CreateListRole(r models.ListRole) (models.ListRole, error) {
	var out models.ListRole

	if err := c.q.CreateRole.Get(&out, r.Name, models.RoleTypeList, pq.Array([]string{})); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	if err := c.UpsertListPermissions(out.ID, r.Lists); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// UpsertListPermissions upserts permission for a role.
func (c *Core) UpsertListPermissions(roleID int, lp []models.ListPermission) error {
	var (
		listIDs   = make([]int, 0, len(lp))
		listPerms = make([][]string, 0, len(lp))
	)
	for _, p := range lp {
		if len(p.Permissions) == 0 {
			continue
		}

		listIDs = append(listIDs, p.ID)

		// For the Postgres array unnesting query to work, all permissions arrays should
		// have equal number of entries. Add "" in case there's only one of either list:get or list:manage
		perms := make([]string, 2)
		copy(perms[:], p.Permissions[:])
		listPerms = append(listPerms, perms)
	}

	if _, err := c.q.UpsertListPermissions.Exec(roleID, pq.Array(listIDs), pq.Array(listPerms)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	return nil
}

// DeleteListPermission deletes a list permission entry from a role.
func (c *Core) DeleteListPermission(roleID, listID int) error {
	if _, err := c.q.DeleteListPermission.Exec(roleID, listID); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Constraint == "users_role_id_fkey" {
			return echo.NewHTTPError(http.StatusBadRequest, c.i18n.T("users.cantDeleteRole"))
		}
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	return nil
}

// UpdateUserRole updates a given role.
func (c *Core) UpdateUserRole(id int, r models.Role) (models.Role, error) {
	var out models.Role

	if err := c.q.UpdateRole.Get(&out, id, r.Name, pq.Array(r.Permissions)); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{users.userRole}", "error", pqErrMsg(err)))
	}

	if out.ID == 0 {
		return out, echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.notFound", "name", "{users.userRole}"))
	}

	return out, nil
}

// UpdateListRole updates a given role.
func (c *Core) UpdateListRole(id int, r models.ListRole) (models.ListRole, error) {
	var out models.ListRole

	if err := c.q.UpdateRole.Get(&out, id, r.Name, pq.Array([]string{})); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{users.listRole}", "error", pqErrMsg(err)))
	}

	if out.ID == 0 {
		return out, echo.NewHTTPError(http.StatusBadRequest, c.i18n.Ts("globals.messages.notFound", "name", "{users.listRole}"))
	}

	if err := c.UpsertListPermissions(out.ID, r.Lists); err != nil {
		return out, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorCreating", "name", "{users.listRole}", "error", pqErrMsg(err)))
	}

	return out, nil
}

// DeleteRole deletes a given role.
func (c *Core) DeleteRole(id int) error {
	if _, err := c.q.DeleteRole.Exec(id); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Constraint == "users_role_id_fkey" {
			return echo.NewHTTPError(http.StatusBadRequest, c.i18n.T("users.cantDeleteRole"))
		}
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "{users.role}", "error", pqErrMsg(err)))
	}

	return nil
}
