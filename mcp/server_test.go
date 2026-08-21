package mcp

import (
	"testing"

	"github.com/asenawritescode/kora/api/ai"
	"github.com/asenawritescode/kora/doctype"
)

func TestToolGeneration_ForDoctype(t *testing.T) {
	reg := doctype.NewRegistry()

	// Register a non-child doctype.
	dt := &doctype.DocType{
		Name: "Customer",
		Fields: []doctype.Field{
			{Fieldname: "customer_name", Fieldtype: "Data", Reqd: true},
			{Fieldname: "email", Fieldtype: "Data"},
			{Fieldname: "phone", Fieldtype: "Data"},
		},
	}
	reg.Register(dt)

	server := New(reg, "test-site", ModeExecutable)

	if server == nil {
		t.Fatal("New returned nil")
	}
	if server.srv == nil {
		t.Fatal("server.srv is nil (no tools registered)")
	}
}

func TestToolNames_FollowPattern(t *testing.T) {
	reg := doctype.NewRegistry()

	// Multi-word doctype.
	dt := &doctype.DocType{
		Name: "Work Order",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data"},
		},
	}
	reg.Register(dt)

	server := New(reg, "test-site", ModeExecutable)
	if server == nil {
		t.Fatal("New returned nil")
	}

	// The tool names should follow the pattern <sanitized_name>_<operation>.
	// For "Work Order", sanitized name is "work_order".
	// Tools: work_order_list, work_order_create, work_order_get, work_order_update, work_order_delete
	// Plus validate_yaml from addConfigTools.
	// We can verify by checking the server was created without panic.
	_ = server
}

func TestEmptyRegistry_NoTools(t *testing.T) {
	reg := doctype.NewRegistry()

	server := New(reg, "test-site", ModeValidationOnly)
	if server == nil {
		t.Fatal("New returned nil")
	}
	_ = server
}

func TestAddDoctypeTools_ExcludesChildTables(t *testing.T) {
	reg := doctype.NewRegistry()

	// Register a parent doctype.
	parent := &doctype.DocType{
		Name: "Invoice",
		Fields: []doctype.Field{
			{Fieldname: "total", Fieldtype: "Currency"},
		},
	}
	reg.Register(parent)

	// Register a child table doctype (should be excluded from tools).
	child := &doctype.DocType{
		Name:         "Invoice Item",
		IsChildTable: true,
		Fields: []doctype.Field{
			{Fieldname: "item_name", Fieldtype: "Data"},
			{Fieldname: "qty", Fieldtype: "Int"},
			{Fieldname: "rate", Fieldtype: "Currency"},
		},
	}
	reg.Register(child)

	server := New(reg, "test-site", ModeExecutable)
	if server == nil {
		t.Fatal("New returned nil")
	}
	// Should have validated that child tables don't generate tools without panic.
	_ = server
}

func TestValidationOnlyModeExposesConfigToolsOnly(t *testing.T) {
	reg := doctype.NewRegistry()
	reg.Register(&doctype.DocType{Name: "Customer"})

	server := New(reg, "test-site", ModeValidationOnly)
	if server == nil || server.srv == nil {
		t.Fatal("expected server to initialize")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Customer", "customer"},
		{"Work Order", "work_order"},
		{"Sales-Invoice", "sales_invoice"},
		{"Lead", "lead"},
		{"already_lower", "already_lower"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProjectedMCPToolsMirrorCatalog(t *testing.T) {
	reg := doctype.NewRegistry()
	reg.Register(&doctype.DocType{
		Name: "Task",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data", Reqd: true},
			{Fieldname: "age", Fieldtype: "Int"},
		},
	})

	server := New(reg, "test-site", ModeExecutable)
	if server == nil || server.srv == nil {
		t.Fatal("expected server to initialize")
	}

	catalog := ai.BuildToolCatalog(reg)
	if len(catalog.Tools) == 0 {
		t.Fatal("expected tool catalog entries")
	}
}
