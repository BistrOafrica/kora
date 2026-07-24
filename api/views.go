package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/asenawritescode/kora/configstore"
	"github.com/asenawritescode/kora/doctype"
)

// --- System View CRUD ---

// HandleSystemViews returns all views for the current site.
// GET /api/v1/system/views
func (h *Handler) HandleSystemViews(c *gin.Context) {
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "view store not available"}})
		return
	}

	views, err := store.LoadViews(site)
	if err != nil {
		internalError(c, "loading views", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: views})
}

// HandleSystemView returns a single view by name.
// GET /api/v1/system/views/:name
func (h *Handler) HandleSystemView(c *gin.Context) {
	name := c.Param("name")
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "view store not available"}})
		return
	}

	view, err := store.LoadView(name, site)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found: " + name}})
		return
	}

	// YAML export format.
	if c.Query("format") == "yaml" {
		yamlBytes, err := yaml.Marshal(view)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to serialize YAML"}})
			return
		}
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", yamlBytes)
		return
	}

	c.JSON(http.StatusOK, Response{Data: view})
}

// HandleSystemViewCreate creates a new view and returns a config version.
// POST /api/v1/system/views
func (h *Handler) HandleSystemViewCreate(c *gin.Context) {
	var view doctype.View
	if err := c.ShouldBindJSON(&view); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid view JSON: " + err.Error()}})
		return
	}

	if err := view.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "view store not available"}})
		return
	}

	if err := store.SaveView(&view, site); err != nil {
		internalError(c, "saving view", err)
		return
	}

	// Create a config version for this change.
	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}

	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Created view "+view.Name, "Draft", snapshot)
	if err != nil {
		internalError(c, "creating config version", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"view":        view,
		"version_id":  versionID,
		"version_num": versionNum,
		"status":      "Draft",
	}})
}

// HandleSystemViewUpdate updates an existing view and returns a config version.
// PUT /api/v1/system/views/:name
func (h *Handler) HandleSystemViewUpdate(c *gin.Context) {
	name := c.Param("name")
	var view doctype.View
	if err := c.ShouldBindJSON(&view); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid view JSON: " + err.Error()}})
		return
	}

	// Enforce name consistency.
	view.Name = name

	if err := view.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "view store not available"}})
		return
	}

	if err := store.SaveView(&view, site); err != nil {
		internalError(c, "saving view", err)
		return
	}

	// Create a config version.
	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}

	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Updated view "+view.Name, "Draft", snapshot)
	if err != nil {
		internalError(c, "creating config version", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"view":        view,
		"version_id":  versionID,
		"version_num": versionNum,
		"status":      "Draft",
	}})
}

// HandleSystemViewDelete removes a view and returns a config version.
// DELETE /api/v1/system/views/:name
func (h *Handler) HandleSystemViewDelete(c *gin.Context) {
	name := c.Param("name")
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "view store not available"}})
		return
	}

	if err := store.DeleteView(name, site); err != nil {
		internalError(c, "deleting view", err)
		return
	}

	// Create a config version.
	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}

	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Deleted view "+name, "Draft", snapshot)
	if err != nil {
		internalError(c, "creating config version", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"version_id":  versionID,
		"version_num": versionNum,
		"status":      "Draft",
	}})
}

// HandleViewValidate validates a view config against the registry's doctypes.
// POST /api/v1/system/views/validate
func (h *Handler) HandleViewValidate(c *gin.Context) {
	var view doctype.View
	if err := c.ShouldBindJSON(&view); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid view JSON: " + err.Error()}})
		return
	}

	// Structural validation first.
	if err := view.Validate(); err != nil {
		c.JSON(http.StatusOK, Response{Data: map[string]any{
			"valid":  false,
			"errors": []map[string]string{{"message": err.Error()}},
		}})
		return
	}

	// Validate against registry doctypes.
	reg := h.siteRegistry(c)
	errors := validateViewAgainstRegistry(&view, reg)

	if len(errors) > 0 {
		errMsgs := make([]map[string]string, len(errors))
		for i, e := range errors {
			errMsgs[i] = map[string]string{"message": e}
		}
		c.JSON(http.StatusOK, Response{Data: map[string]any{
			"valid":  false,
			"errors": errMsgs,
		}})
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"valid": true,
	}})
}

// validateViewAgainstRegistry checks that component bindings reference real doctypes and fields.
func validateViewAgainstRegistry(view *doctype.View, reg *doctype.Registry) []string {
	var errors []string

	for i := range view.Components {
		comp := &view.Components[i]
		errors = append(errors, validateComponentAgainstRegistry(comp, reg)...)
	}

	return errors
}

func validateComponentAgainstRegistry(comp *doctype.ViewComponent, reg *doctype.Registry) []string {
	var errors []string
	prefix := fmt.Sprintf("component %q", comp.ID)

	// Check source doctype exists.
	if comp.SourceDocType != "" {
		if dt := reg.Get(comp.SourceDocType); dt == nil {
			errors = append(errors, fmt.Sprintf("%s: source doctype %q not found", prefix, comp.SourceDocType))
		} else {
			// Check bindings reference real fields.
			for prop, fieldName := range comp.Bindings {
				if f := dt.GetField(fieldName); f == nil {
					// Check if it's a system field.
					if !isPublicSystemField(fieldName) {
						errors = append(errors, fmt.Sprintf("%s: binding %q references unknown field %q on %s",
							prefix, prop, fieldName, comp.SourceDocType))
					}
				}
			}
		}
	}

	// Validate nested children.
	for i := range comp.Components {
		errors = append(errors, validateComponentAgainstRegistry(&comp.Components[i], reg)...)
	}

	return errors
}

func isPublicSystemField(name string) bool {
	switch name {
	case "name", "owner", "creation", "modified", "modified_by", "doc_status", "idx":
		return true
	default:
		return false
	}
}

// --- View Route Resolution ---

// HandleViewByRoute resolves a view by its route for authenticated users.
// GET /api/v1/views?route=/pos/register
// Optional: ?version=draft to preview the latest Draft version.
func (h *Handler) HandleViewByRoute(c *gin.Context) {
	route := c.Query("route")
	if route == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "route query parameter is required"}})
		return
	}

	// Normalize: ensure leading slash.
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}

	// Draft preview: load from latest Draft config snapshot.
	if c.Query("version") == "draft" {
		store := h.viewStore(c)
		if store != nil {
			site := siteName(c)
			var configJSON string
			err := store.DB.QueryRow(
				"SELECT config FROM _kora_config_version WHERE site = ? AND status = 'Draft' ORDER BY version DESC LIMIT 1",
				site,
			).Scan(&configJSON)
			if err == nil && configJSON != "" {
				snapshot, parseErr := doctype.ParseConfig(configJSON)
				if parseErr == nil {
					for _, v := range snapshot.Views {
						if v.Route == route {
							c.JSON(http.StatusOK, Response{Data: map[string]any{
								"view":      v,
								"is_public": false,
								"draft":     true,
							}})
							return
						}
					}
				}
			}
		}
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "No draft version found for route: " + route}})
		return
	}

	reg := h.siteRegistry(c)
	view := reg.Views.GetByRoute(route)
	if view == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found for route: " + route}})
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"view":      view,
		"is_public": false,
	}})
}

// --- Public Views ---

// HandlePublicView resolves a view by route for unauthenticated users.
// GET /v?route=/catalog
// Three-layer security check:
//  1. View allows public access for the component
//  2. Component's source doctype has public access enabled
//  3. Component bindings only reference public fields
func (h *Handler) HandlePublicView(c *gin.Context) {
	route := c.Query("route")
	if route == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "route query parameter is required"}})
		return
	}

	// Normalize: ensure leading slash.
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}

	reg := h.siteRegistry(c)
	view := reg.Views.GetByRoute(route)
	if view == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found"}})
		return
	}

	// Layer 1: View allows public access.
	if view.PublicAccess == nil || !view.PublicAccess.Enabled {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found"}})
		return
	}

	// Filter and validate components.
	filtered := make([]doctype.ViewComponent, 0)
	for _, comp := range view.Components {
		// Check if this component is in the allowed set.
		if !view.PublicAccess.AllowsComponent(comp.ID) {
			continue
		}

		// Layer 2: Source doctype must have public access enabled.
		if comp.SourceDocType != "" {
			dt := reg.Get(comp.SourceDocType)
			if dt == nil || dt.PublicAccess == nil || !dt.PublicAccess.Enabled {
				continue
			}

			// Layer 3: Strip bindings that reference non-public fields.
			publicFields := dt.PublicFieldSet()
			if comp.Bindings != nil {
				filteredBindings := make(map[string]string)
				for prop, fieldName := range comp.Bindings {
					if publicFields[fieldName] || isPublicSystemField(fieldName) {
						filteredBindings[prop] = fieldName
					}
				}
				comp.Bindings = filteredBindings
			}
		}

		// Strip mutation actions unless explicitly allowed.
		if !view.PublicAccess.AllowMutations {
			var safeActions []doctype.ViewAction
			for _, action := range comp.Actions {
				if !action.IsMutation() {
					safeActions = append(safeActions, action)
				}
			}
			comp.Actions = safeActions
		}

		filtered = append(filtered, comp)
	}

	// Return stripped view config.
	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"view":       view,
		"components": filtered,
		"is_public":  true,
	}})
}

// --- Helpers ---

// ---------------------------------------------------------------------------
// View Action Execution
// ---------------------------------------------------------------------------

// HandleViewAction executes a view action server-side.
// POST /api/v1/view/action/:actionId
// The server resolves the action type and config from the stored view config.
// The client sends only context data — never chooses the action type.
func (h *Handler) HandleViewAction(c *gin.Context) {
	actionID := c.Param("actionId")

	var req struct {
		View      string         `json:"view"`
		Component string         `json:"component"`
		Context   map[string]any `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request: " + err.Error()}})
		return
	}

	if req.View == "" || req.Component == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "view and component are required"}})
		return
	}

	reg := h.siteRegistry(c)
	view := reg.Views.GetByName(req.View)
	if view == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found: " + req.View}})
		return
	}

	// Find the component and action in the stored view config.
	var targetAction *doctype.ViewAction
	for i := range view.Components {
		if view.Components[i].ID == req.Component {
			for j := range view.Components[i].Actions {
				if view.Components[i].Actions[j].ID == actionID {
					targetAction = &view.Components[i].Actions[j]
					break
				}
			}
			break
		}
	}

	if targetAction == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{
			"message": fmt.Sprintf("Action %q not found on component %q in view %q", actionID, req.Component, req.View),
		}})
		return
	}

	// Execute the action based on resolved type (from stored config, not client).
	switch targetAction.Type {
	case "create_record":
		h.executeCreateRecord(c, targetAction, req.Context)
	case "update_record":
		h.executeUpdateRecord(c, targetAction, req.Context)
	case "workflow_transition":
		h.executeWorkflowTransition(c, targetAction, req.Context)
	case "create_transaction":
		h.executeCreateTransaction(c, targetAction, req.Context)
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{
			"message": fmt.Sprintf("Action type %q must be executed client-side", targetAction.Type),
		}})
	}
}

func (h *Handler) executeCreateRecord(c *gin.Context, action *doctype.ViewAction, ctx map[string]any) {
	doctypeName := getString(action.Config, "target_doctype")
	if doctypeName == "" {
		doctypeName = getString(ctx, "_doctype")
	}
	if doctypeName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "target_doctype is required"}})
		return
	}

	dt := h.siteRegistry(c).Get(doctypeName)
	if dt == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not found: " + doctypeName}})
		return
	}

	doc := doctype.NewDocument("")
	for k, v := range ctx {
		if !strings.HasPrefix(k, "_") {
			doc.Set(k, v)
		}
	}

	user := currentUser(c)
	if err := h.siteTx(c).Insert(dt, doc, user, "view-action"); err != nil {
		handleViewError(c, dt, err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: documentToMap(doc, dt)})
}

func (h *Handler) executeUpdateRecord(c *gin.Context, action *doctype.ViewAction, ctx map[string]any) {
	doctypeName := getString(action.Config, "target_doctype")
	if doctypeName == "" {
		doctypeName = getString(ctx, "_doctype")
	}
	name := getString(ctx, "name")
	if doctypeName == "" || name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "target_doctype and name are required"}})
		return
	}

	dt := h.siteRegistry(c).Get(doctypeName)
	if dt == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not found: " + doctypeName}})
		return
	}

	tm := h.siteTx(c)
	existing, err := tm.GetDoc(dt, name, "")
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Document not found: " + name}})
		return
	}

	for k, v := range ctx {
		if !strings.HasPrefix(k, "_") && k != "name" {
			existing.Set(k, v)
		}
	}

	user := currentUser(c)
	if err := tm.Save(dt, existing, user, "view-action", nil); err != nil {
		handleViewError(c, dt, err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: documentToMap(existing, dt)})
}

func (h *Handler) executeWorkflowTransition(c *gin.Context, action *doctype.ViewAction, ctx map[string]any) {
	transition := getString(action.Config, "transition")
	doctypeName := getString(action.Config, "doctype")
	name := getString(ctx, "name")

	if doctypeName == "" || name == "" || transition == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "doctype, name, and transition are required"}})
		return
	}

	reg := h.siteRegistry(c)
	wf := reg.Workflows.Get(doctypeName)
	if wf == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "No workflow for doctype: " + doctypeName}})
		return
	}

	dt := reg.Get(doctypeName)
	if dt == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not found: " + doctypeName}})
		return
	}

	tm := h.siteTx(c)
	doc, err := tm.GetDoc(dt, name, "")
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Document not found: " + name}})
		return
	}

	currentState := fmt.Sprintf("%v", doc.Get(wf.WorkflowStateField))
	user := currentUser(c)
	userRole := "Administrator"
	if r, ok := c.Get("user_role"); ok {
		if s, ok := r.(string); ok {
			userRole = s
		}
	}

	newState, newDocStatus, err := reg.Workflows.ApplyTransition(doctypeName, currentState, transition, userRole, doc)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	doc.Set(wf.WorkflowStateField, newState)
	doc.Set("doc_status", newDocStatus)

	if err := tm.Save(dt, doc, user, "view-action", nil); err != nil {
		handleViewError(c, dt, err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: documentToMap(doc, dt)})
}

func (h *Handler) executeCreateTransaction(c *gin.Context, action *doctype.ViewAction, ctx map[string]any) {
	targetDoctype := getString(action.Config, "target_doctype")
	if targetDoctype == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "target_doctype is required"}})
		return
	}

	reg := h.siteRegistry(c)
	dt := reg.Get(targetDoctype)
	if dt == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not found: " + targetDoctype}})
		return
	}

	user := currentUser(c)
	tm := h.siteTx(c)

	// Build parent document from context (excluding cart/items transport fields).
	doc := doctype.NewDocument("")
	for k, v := range ctx {
		if k != "cart" && k != "items" && k != "total" && !strings.HasPrefix(k, "_") {
			doc.Set(k, v)
		}
	}

	parentField, childDT, err := resolveTransactionChildTable(reg, dt, action)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	if childDT != nil {
		rawItems := ctx["cart"]
		if rawItems == nil {
			rawItems = ctx["items"]
		}
		children, err := buildTransactionChildren(rawItems, childDT)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
			return
		}
		if len(children) == 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "transaction requires at least one item"}})
			return
		}
		doc.SetTable(parentField, children)
	}

	if err := tm.Insert(dt, doc, user, "view-action"); err != nil {
		handleViewError(c, dt, err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: documentToMap(doc, dt)})
}

func resolveTransactionChildTable(reg *doctype.Registry, parentDT *doctype.DocType, action *doctype.ViewAction) (string, *doctype.DocType, error) {
	configured := getString(action.Config, "child_table")
	parentField := getString(action.Config, "parent_field")

	for _, field := range parentDT.TableFields() {
		if parentField != "" && field.Fieldname != parentField {
			continue
		}
		if configured == "" || configured == field.Fieldname || configured == field.Options {
			childDT := reg.Get(field.Options)
			if childDT == nil {
				return "", nil, fmt.Errorf("child doctype %q not found", field.Options)
			}
			return field.Fieldname, childDT, nil
		}
	}

	if configured == "" {
		return "", nil, nil
	}
	return "", nil, fmt.Errorf("child_table %q is not a table field or child doctype on %s", configured, parentDT.Name)
}


func buildTransactionChildren(rawItems any, childDT *doctype.DocType) ([]*doctype.Document, error) {
	items, ok := rawItems.([]any)
	if !ok {
		return nil, fmt.Errorf("cart/items must be an array")
	}

	children := make([]*doctype.Document, 0, len(items))
	for i, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("item %d must be an object", i+1)
		}

		child := doctype.NewDocument("")
		for _, field := range childDT.NonTableDataFields() {
			if field.ReadOnly && field.Computed != "" {
				continue
			}
			if val, ok := row[field.Fieldname]; ok {
				child.Set(field.Fieldname, val)
				continue
			}
			switch field.Fieldname {
			case "product":
				if val, ok := firstPresent(row, "product", "name"); ok {
					child.Set(field.Fieldname, val)
				}
			case "unit_price":
				if val, ok := firstPresent(row, "unit_price", "rate", "price", "selling_price"); ok {
					child.Set(field.Fieldname, val)
				}
			case "quantity":
				if val, ok := firstPresent(row, "quantity", "qty"); ok {
					child.Set(field.Fieldname, val)
				} else {
					child.Set(field.Fieldname, 1)
				}
			}
		}
		children = append(children, child)
	}
	return children, nil
}

func firstPresent(row map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if val, ok := row[key]; ok && val != nil {
			return val, true
		}
	}
	return nil, false
}

func findViewComponentByID(components []doctype.ViewComponent, id string) *doctype.ViewComponent {
	for i := range components {
		if components[i].ID == id {
			return &components[i]
		}
		if child := findViewComponentByID(components[i].Components, id); child != nil {
			return child
		}
	}
	return nil
}

// HandleViewData returns aggregated data for dashboard/metric components.
// GET /api/v1/view/data?view=Name&component=id
func (h *Handler) HandleViewData(c *gin.Context) {
	viewName := c.Query("view")
	componentID := c.Query("component")

	if viewName == "" || componentID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "view and component are required"}})
		return
	}

	reg := h.siteRegistry(c)
	view := reg.Views.GetByName(viewName)
	if view == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found: " + viewName}})
		return
	}

	// Find the component to get its source doctype. Components can be nested
	// inside containers such as dashboard_grid, tabs, or split_view.
	comp := findViewComponentByID(view.Components, componentID)
	if comp == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Component not found: " + componentID}})
		return
	}

	// For metric_card components, return count/aggregate.
	doctypeName := comp.SourceDocType
	if doctypeName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Component has no source_doctype"}})
		return
	}

	dt := reg.Get(doctypeName)
	if dt == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not found: " + doctypeName}})
		return
	}

	// Get count via ORM.
	statusFilter := ""
	if comp.Bindings != nil {
		if s, ok := comp.Bindings["status"]; ok {
			statusFilter = s
		}
	}

	_, total, err := h.siteTx(c).GetList(dt, statusFilter, "", 1, 0, "")
	if err != nil {
		c.JSON(http.StatusOK, Response{Data: map[string]any{
			"count": 0,
			"label": comp.Label,
		}})
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"count": total,
		"label": comp.Label,
	}})
}

// HandlePublicCreate handles unauthenticated document creation.
// POST /v?route=/apply
func (h *Handler) HandlePublicCreate(c *gin.Context) {
	route := c.Query("route")
	if route == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "route query parameter is required"}})
		return
	}

	reg := h.siteRegistry(c)
	view := reg.Views.GetByRoute(route)
	if view == nil || view.PublicAccess == nil || !view.PublicAccess.Enabled || !view.PublicAccess.AllowMutations {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "View not found or public mutations not allowed"}})
		return
	}

	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid JSON: " + err.Error()}})
		return
	}

	// Use the view's source doctype.
	if view.SourceDocType == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "View has no source doctype"}})
		return
	}

	dt := reg.Get(view.SourceDocType)
	if dt == nil || dt.PublicAccess == nil || !dt.PublicAccess.Enabled {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Doctype not public"}})
		return
	}

	// Strip non-public fields.
	publicFields := dt.PublicFieldSet()
	doc := &doctype.Document{}
	for k, v := range body {
		if publicFields[k] || isPublicSystemField(k) {
			doc.Set(k, v)
		}
	}

	user := "public"
	tm := h.siteTx(c)
	if err := tm.Insert(dt, doc, user, "public-form"); err != nil {
		handleViewError(c, dt, err)
		return
	}

	// Only return public fields.
	result := make(map[string]any)
	for field := range publicFields {
		result[field] = doc.Get(field)
	}
	result["name"] = doc.Name

	c.JSON(http.StatusOK, Response{Data: result})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// viewStore returns a config store for the current site, or nil if unavailable.
func (h *Handler) viewStore(c *gin.Context) *configstore.Store {
	db, _ := c.Get("site_db")
	if db == nil {
		return nil
	}
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return nil
	}
	return configstore.NewStore(sqlDB, h.TxManager.Dialect)
}

// currentUser returns the authenticated user identifier from context.
func currentUser(c *gin.Context) string {
	if user, ok := c.Get("user"); ok {
		if s, ok := user.(string); ok && s != "" {
			return s
		}
	}
	return "system"
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// getString safely gets a string value from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// documentToMap converts a Document to a plain map for JSON responses.
func documentToMap(doc *doctype.Document, dt *doctype.DocType) map[string]any {
	result := make(map[string]any)
	result["name"] = doc.Name
	result["owner"] = doc.Get("owner")
	result["creation"] = doc.Get("creation")
	result["modified"] = doc.Get("modified")
	result["modified_by"] = doc.Get("modified_by")
	result["doc_status"] = doc.DocStatus
	for _, f := range dt.DataFields() {
		if f.Fieldtype != "Table" {
			result[f.Fieldname] = doc.Get(f.Fieldname)
		}
	}
	return result
}

// handleViewError maps ORM/database errors to API error responses.
func handleViewError(c *gin.Context, dt *doctype.DocType, err error) {
	if ve, ok := err.(*doctype.ValidationError); ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{
			"type":    "ValidationError",
			"message": ve.Error(),
			"field":   ve.Field,
		}})
		return
	}
	if strings.Contains(err.Error(), "ValidationError") {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{
			"type":    "ValidationError",
			"message": err.Error(),
		}})
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{
		"message": err.Error(),
	}})
}
