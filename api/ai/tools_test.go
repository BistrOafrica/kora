package ai

import (
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

	system := findTool(t, catalog.Tools, "script_create")
	if !allowsChannel(system.ChannelAllowlist, "whatsapp") {
		t.Fatalf("script_create should be allowed on whatsapp for beta parity")
	}
	if !system.RequiresConfirmation || !system.RequiresRecentAuth {
		t.Fatalf("script_create should remain guarded: %#v", system)
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
