package orm

import (
	"testing"

	"github.com/asenawritescode/kora/doctype"
)

func TestNormalizeHookDocumentFields_RestoresChildTables(t *testing.T) {
	reg := doctype.NewRegistry()
	reg.Register(&doctype.DocType{Name: "Sale Item", IsChildTable: true, Fields: []doctype.Field{
		{Fieldname: "product", Fieldtype: "Link", Options: "Product"},
		{Fieldname: "quantity", Fieldtype: "Float"},
	}})
	sale := &doctype.DocType{Name: "Sale", Fields: []doctype.Field{
		{Fieldname: "receipt_number", Fieldtype: "Data"},
		{Fieldname: "items", Fieldtype: "Table", Options: "Sale Item"},
	}}

	fields := normalizeHookDocumentFields(sale, reg, map[string]any{
		"name":           "SALE-0001",
		"doc_status":     float64(0),
		"receipt_number": "RCPT-1",
		"items": []any{
			map[string]any{"name": "SI-0001", "product": "PROD-0001", "quantity": float64(2)},
		},
	})
	doc := &doctype.Document{DocType: "Sale", Fields: fields}

	if _, ok := fields["name"]; ok {
		t.Fatalf("system field name should not be copied into document fields")
	}
	items := doc.GetTable("items")
	if len(items) != 1 {
		t.Fatalf("items count: got %d, want 1 (%#v)", len(items), fields["items"])
	}
	if got := items[0].Name; got != "SI-0001" {
		t.Fatalf("items[0].name: got %q, want SI-0001", got)
	}
	if got := items[0].Get("product"); got != "PROD-0001" {
		t.Fatalf("items[0].product: got %#v, want PROD-0001", got)
	}
}
