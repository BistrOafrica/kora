package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/asenawritescode/kora/doctype"
)

func TestBuildToolCatalogAllowsWhatsAppWithGuards(t *testing.T) {
	reg := doctype.NewRegistry()
	reg.Register(&doctype.DocType{
		Name: "Customer",
		Fields: []doctype.Field{
			{Fieldname: "name", Fieldtype: "Data", Label: "Name", Reqd: true},
			{Fieldname: "email", Fieldtype: "Data", Label: "Email"},
		},
	})

	catalog := BuildToolCatalog(reg)
	create := findTool(t, catalog.Tools, "customer_create")
	if !allowsChannel(create.ChannelAllowlist, "whatsapp") {
		t.Fatalf("customer_create should be allowed on whatsapp: %#v", create.ChannelAllowlist)
	}
	if !create.RequiresConfirmation || !create.RequiresRecentAuth {
		t.Fatalf("customer_create should remain guarded: %#v", create)
	}
	update := findTool(t, catalog.Tools, "customer_update")
	if !allowsChannel(update.ChannelAllowlist, "whatsapp") {
		t.Fatalf("customer_update should be allowed on whatsapp: %#v", update.ChannelAllowlist)
	}
	if !update.RequiresConfirmation || !update.RequiresRecentAuth || update.Operation != "update" || update.Doctype != "Customer" {
		t.Fatalf("customer_update should be guarded update metadata: %#v", update)
	}
	props := update.InputSchema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Fatalf("customer_update should require stable document name: %#v", update.InputSchema)
	}
	required := update.InputSchema["required"].([]string)
	if len(required) != 1 || required[0] != "name" {
		t.Fatalf("customer_update should only require name: %#v", update.InputSchema)
	}

	system := findTool(t, catalog.Tools, "script_create")
	if !allowsChannel(system.ChannelAllowlist, "whatsapp") {
		t.Fatalf("script_create should be allowed on whatsapp for beta parity")
	}
	if !system.RequiresConfirmation || !system.RequiresRecentAuth {
		t.Fatalf("script_create should remain guarded: %#v", system)
	}
}

func TestBuildToolCatalogEmitsV2FindSchemaAndMetadata(t *testing.T) {
	reg := doctype.NewRegistry()
	reg.Register(&doctype.DocType{
		Name:         "Task",
		TitleField:   "title",
		SearchFields: "title, status",
		SortField:    "modified",
		SortOrder:    "DESC",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data", Label: "Title", Reqd: true, SearchIndex: true},
			{Fieldname: "due_date", Fieldtype: "Date", Label: "Due Date", InStandardFilter: true},
			{Fieldname: "status", Fieldtype: "Select", Label: "Status", Options: "Todo\nDone"},
		},
	})

	catalog := BuildToolCatalog(reg)
	find := findTool(t, catalog.Tools, "task_find")
	if find.ArgumentContractVersion != "v2" || find.Operation != "find" || find.Doctype != "Task" {
		t.Fatalf("expected v2 find metadata, got %#v", find)
	}
	props := find.InputSchema["properties"].(map[string]any)
	if _, ok := props["filters"]; !ok {
		t.Fatalf("expected filters property in find schema: %#v", props)
	}
	if _, ok := props["due_date"]; ok {
		t.Fatalf("find schema should not expose flat field args: %#v", props)
	}
	if len(find.FieldHints) != 3 || len(find.SystemFields) == 0 {
		t.Fatalf("expected field and system hints, got %#v", find)
	}
}

func TestListFormattingUsesSchemaSummary(t *testing.T) {
	dt := &doctype.DocType{
		Name:       "Task",
		TitleField: "title",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data", Label: "Title", InListView: true},
			{Fieldname: "due_date", Fieldtype: "Date", Label: "Due Date", InListView: true},
			{Fieldname: "priority", Fieldtype: "Select", Label: "Priority", InListView: true, Options: "Low\nMedium\nHigh"},
			{Fieldname: "creation", Fieldtype: "Datetime", Label: "Creation"},
		},
	}
	doc := doctype.NewDocument("Task")
	doc.Name = "TASK-0001"
	doc.Fields = map[string]any{
		"title":    "Call home",
		"due_date": "2026-07-14 00:00:00 +0000 UTC",
		"priority": "Medium",
		"creation": "2026-07-13 20:49:53 +0000 UTC",
	}
	out := formatDocSummary(dt, doc, 1)

	if strings.Contains(out, "map[") || strings.Contains(out, "+0000 UTC") || strings.Contains(out, "Creation") {
		t.Fatalf("expected clean schema summary, got %q", out)
	}
	for _, want := range []string{"1. Call home", "ID: TASK-0001", "Due Date: 2026-07-14", "Priority: Medium"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in summary %q", want, out)
		}
	}
}

func TestBuildValidatedFindArgsRejectsInvalidFindInputs(t *testing.T) {
	dt := &doctype.DocType{
		Name: "Task",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data", Label: "Title"},
			{Fieldname: "due_date", Fieldtype: "Date", Label: "Due Date"},
			{Fieldname: "priority", Fieldtype: "Int", Label: "Priority"},
		},
	}

	if _, _, _, _, err := buildValidatedFindArgs(dt, map[string]any{"due_date": "2026-07-15"}); err == nil {
		t.Fatal("expected flat field args to be rejected")
	}
	if _, _, _, _, err := buildValidatedFindArgs(dt, map[string]any{
		"filters": []any{map[string]any{"field": "missing", "op": "=", "value": "x"}},
	}); err == nil {
		t.Fatal("expected unknown filter field to be rejected")
	}
	if _, _, _, _, err := buildValidatedFindArgs(dt, map[string]any{
		"filters": []any{map[string]any{"field": "priority", "op": "like", "value": "1"}},
	}); err == nil {
		t.Fatal("expected invalid numeric operator to be rejected")
	}
	if _, _, _, _, err := buildValidatedFindArgs(dt, map[string]any{
		"filters":  []any{map[string]any{"field": "due_date", "op": "=", "value": "2026-07-15"}},
		"order_by": "missing DESC",
	}); err == nil {
		t.Fatal("expected invalid order_by to be rejected")
	}
}

func TestBuildValidatedFindArgsBuildsTypedORMFilters(t *testing.T) {
	dt := &doctype.DocType{
		Name: "Task",
		Fields: []doctype.Field{
			{Fieldname: "due_date", Fieldtype: "Date", Label: "Due Date"},
			{Fieldname: "priority", Fieldtype: "Int", Label: "Priority"},
		},
	}

	filter, limit, offset, orderBy, err := buildValidatedFindArgs(dt, map[string]any{
		"filters": []any{
			map[string]any{"field": "due_date", "op": "=", "value": "2026-07-15"},
			map[string]any{"field": "priority", "op": ">=", "value": "2"},
		},
		"limit":    float64(7),
		"offset":   float64(3),
		"order_by": "modified DESC",
	})
	if err != nil {
		t.Fatal(err)
	}
	var parsed [][]any
	if err := json.Unmarshal([]byte(filter), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[0][0] != "due_date" || parsed[0][2] != "2026-07-15" || parsed[1][0] != "priority" || parsed[1][1] != ">=" || parsed[1][2] != float64(2) || limit != 7 || offset != 3 || orderBy != "modified DESC" {
		t.Fatalf("unexpected find args: filter=%s limit=%d offset=%d order=%q", filter, limit, offset, orderBy)
	}
}

func findTool(t *testing.T, tools []ToolDescriptor, name string) ToolDescriptor {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %s not found", name)
	return ToolDescriptor{}
}

func allowsChannel(values []string, channel string) bool {
	for _, value := range values {
		if value == channel {
			return true
		}
	}
	return false
}
