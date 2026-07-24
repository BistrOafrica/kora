package api

import (
	"testing"

	"github.com/asenawritescode/kora/doctype"
)

func TestCloneScriptDocument_PreservesOldChildTableState(t *testing.T) {
	original := &doctype.Document{
		DocType: "Product",
		Name:    "PROD-0001",
		Fields: map[string]any{
			"product_name": "Bananas",
			"stock_moves": []*doctype.Document{
				{DocType: "Stock Move", Name: "MOVE-0001", Fields: map[string]any{"qty_change": float64(80)}},
			},
		},
		DocStatus: 0,
	}

	oldDoc := cloneScriptDocument(original)
	original.GetTable("stock_moves")[0].Set("qty_change", float64(75))
	original.SetTable("stock_moves", append(original.GetTable("stock_moves"), &doctype.Document{DocType: "Stock Move", Fields: map[string]any{"qty_change": float64(-2)}}))

	oldMoves := oldDoc.GetTable("stock_moves")
	if len(oldMoves) != 1 {
		t.Fatalf("old stock move count: got %d, want 1", len(oldMoves))
	}
	if got := oldMoves[0].Get("qty_change"); got != float64(80) {
		t.Fatalf("old stock move qty: got %#v, want 80", got)
	}
	if len(original.GetTable("stock_moves")) != 2 {
		t.Fatalf("new stock move count: got %d, want 2", len(original.GetTable("stock_moves")))
	}
}

func TestDocumentToMap_ExposesChildRowsAsPlainMaps(t *testing.T) {
	doc := &doctype.Document{
		DocType: "Sale",
		Name:    "SALE-0001",
		Fields: map[string]any{
			"items": []*doctype.Document{
				{DocType: "Sale Item", Name: "SI-0001", Fields: map[string]any{"product": "PROD-0001", "quantity": float64(2)}},
			},
		},
	}

	mapped := doc.ToMap()
	items, ok := mapped["items"].([]map[string]any)
	if !ok {
		t.Fatalf("items type: got %T, want []map[string]any", mapped["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items count: got %d, want 1", len(items))
	}
	if got := items[0]["product"]; got != "PROD-0001" {
		t.Fatalf("items[0].product: got %#v, want PROD-0001", got)
	}
	if got := items[0]["name"]; got != "SI-0001" {
		t.Fatalf("items[0].name: got %#v, want SI-0001", got)
	}
}
