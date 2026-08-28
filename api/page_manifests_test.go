package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/asenawritescode/kora/doctype"
	"github.com/gin-gonic/gin"
)

func TestPageManifestEquivalent_UsesSemanticNormalization(t *testing.T) {
	a := &doctype.PageManifest{
		APIVersion: "ui.kora.dev/v1",
		Kind:       "Page",
		Metadata: doctype.PageManifestMetadata{
			Name:    "demos-table",
			Version: "0.1.0",
			Package: "tenant.workspace",
			Status:  "draft",
		},
		Spec: doctype.PageManifestSpec{
			Route:        "/demos",
			Runtime:      ">=2.0.0 <3.0.0",
			Offline:      "read_only",
			Layout:       doctype.PageManifestLayout{Type: "table", Columns: 12},
			Permissions:  nil,
			Capabilities: nil,
			Resources:    nil,
			Actions:      nil,
		},
	}

	b := &doctype.PageManifest{
		APIVersion: "ui.kora.dev/v1",
		Kind:       "Page",
		Metadata: doctype.PageManifestMetadata{
			Name:    "demos-table",
			Version: "0.1.0",
			Package: "tenant.workspace",
			Status:  "draft",
		},
		Spec: doctype.PageManifestSpec{
			Route:        "/demos",
			Runtime:      ">=2.0.0 <3.0.0",
			Offline:      "read_only",
			Layout:       doctype.PageManifestLayout{Type: "table", Columns: 12, Children: []doctype.PageComponent{}},
			Permissions:  []string{},
			Capabilities: []string{},
			Resources:    []doctype.PageResource{},
			Actions:      []doctype.PageAction{},
		},
	}

	if !pageManifestEquivalent(a, b) {
		t.Fatal("expected manifests to be semantically equivalent")
	}

	b.Spec.Layout.Columns = 8
	if pageManifestEquivalent(a, b) {
		t.Fatal("expected manifests with different layout columns to differ")
	}
}

func TestHandleSystemPageManifestUpdate_NoOpDoesNotCreateVersion(t *testing.T) {
	handler, _, mock, sqlDB := setupTestHandler(t)
	gin.SetMode(gin.TestMode)

	requestManifest := doctype.PageManifest{
		APIVersion: "ui.kora.dev/v1",
		Kind:       "Page",
		Metadata: doctype.PageManifestMetadata{
			Name:    "demos-table",
			Version: "0.1.0",
			Package: "tenant.workspace",
			Status:  "draft",
		},
		Spec: doctype.PageManifestSpec{
			Route:   "/demos",
			Runtime: ">=2.0.0 <3.0.0",
			Offline: "read_only",
			Resources: []doctype.PageResource{
				{
					ID:    "primary",
					Query: "document.list",
					Params: map[string]any{
						"doctype": "Demo",
						"limit":   50,
					},
				},
			},
			Layout: doctype.PageManifestLayout{
				Type:     "table",
				Columns:  12,
				Children: []doctype.PageComponent{},
			},
		},
	}

	configBytes, err := json.Marshal(requestManifest)
	if err != nil {
		t.Fatalf("marshal request manifest: %v", err)
	}

	mock.ExpectQuery("SELECT name, route, type, layout, label, module, source_doctype,\\s+public_enabled, public_components, public_allow_mutations, config_json\\s+FROM _kora_view WHERE name = \\? AND \\(site = \\? OR site = ''\\)").
		WithArgs("demos-table", "sync").
		WillReturnRows(ginRowsForPageManifest("demos-table", "/demos", "table", "tenant.workspace", string(configBytes)))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/system/page-manifests/demos-table", bytes.NewBuffer(configBytes))
	c.Params = gin.Params{{Key: "name", Value: "demos-table"}}
	c.Set("site_name", "sync")
	c.Set("site_db", sqlDB)

	handler.HandleSystemPageManifestUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("response data type = %T, want map[string]any", resp.Data)
	}
	if got := data["version_num"]; got != float64(0) {
		t.Fatalf("version_num = %v, want 0", got)
	}
	if got := data["version_id"]; got != "" {
		t.Fatalf("version_id = %v, want empty string", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func ginRowsForPageManifest(name, route, layout, module, configJSON string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"name", "route", "type", "layout", "label", "module", "source_doctype",
		"public_enabled", "public_components", "public_allow_mutations", "config_json",
	}).AddRow(name, route, "page_manifest", layout, name, module, "", 0, nil, 0, configJSON)
}
