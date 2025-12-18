package postgres

import (
	"testing"

	"bib/internal/storage"
)

func TestNewRoleManager(t *testing.T) {
	rm := NewRoleManager("node-123")
	if rm == nil {
		t.Fatal("expected non-nil RoleManager")
	}
	if rm.nodeID != "node-123" {
		t.Errorf("expected node ID 'node-123', got %q", rm.nodeID)
	}
}

func TestGetRoleDefinitions(t *testing.T) {
	defs := GetRoleDefinitions()

	if len(defs) == 0 {
		t.Fatal("expected non-empty role definitions")
	}

	// Check that key roles are defined
	roleNames := make(map[storage.DBRole]bool)
	for _, def := range defs {
		roleNames[def.Name] = true
	}

	expectedRoles := []storage.DBRole{
		storage.RoleScrape,
		storage.RoleQuery,
		storage.RoleTransform,
		storage.RoleAudit,
		storage.RoleReadOnly,
		storage.RoleAdmin,
	}

	for _, role := range expectedRoles {
		if !roleNames[role] {
			t.Errorf("expected role %q to be defined", role)
		}
	}
}

func TestRoleDefinition_Scrape(t *testing.T) {
	defs := GetRoleDefinitions()

	var scrape *RoleDefinition
	for i, def := range defs {
		if def.Name == storage.RoleScrape {
			scrape = &defs[i]
			break
		}
	}

	if scrape == nil {
		t.Fatal("expected scrape role definition")
	}

	if scrape.Description == "" {
		t.Error("expected description for scrape role")
	}
	if !scrape.CanLogin {
		t.Error("expected scrape role to be able to login")
	}
	if scrape.Inherit {
		t.Error("expected scrape role to not inherit")
	}

	// Check table permissions
	if perms, ok := scrape.Tables["datasets"]; !ok {
		t.Error("expected datasets permissions for scrape role")
	} else {
		hasInsert := false
		for _, p := range perms {
			if p == "INSERT" {
				hasInsert = true
				break
			}
		}
		if !hasInsert {
			t.Error("expected INSERT permission on datasets for scrape role")
		}
	}
}

func TestRoleDefinition_Query(t *testing.T) {
	defs := GetRoleDefinitions()

	var query *RoleDefinition
	for i, def := range defs {
		if def.Name == storage.RoleQuery {
			query = &defs[i]
			break
		}
	}

	if query == nil {
		t.Fatal("expected query role definition")
	}

	// Query should only have SELECT
	for table, perms := range query.Tables {
		for _, p := range perms {
			if p != "SELECT" {
				t.Errorf("expected only SELECT for query role on %s, got %s", table, p)
			}
		}
	}
}

func TestRoleDefinition_Admin(t *testing.T) {
	defs := GetRoleDefinitions()

	var admin *RoleDefinition
	for i, def := range defs {
		if def.Name == storage.RoleAdmin {
			admin = &defs[i]
			break
		}
	}

	if admin == nil {
		t.Fatal("expected admin role definition")
	}

	if !admin.CanLogin {
		t.Error("expected admin to be able to login")
	}
	if !admin.Inherit {
		t.Error("expected admin to inherit")
	}
}

func TestRoleManager_GenerateGrantPermissionsSQL(t *testing.T) {
	rm := NewRoleManager("test-node")
	sql := rm.GenerateGrantPermissionsSQL()

	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}

	// Check that SQL contains expected content
	if len(sql) < 100 {
		t.Error("expected substantial SQL output")
	}
}
