package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/asenawritescode/kora/doctype"
)

type pageManifestListEntry struct {
	Name   string `json:"name"`
	Route  string `json:"route"`
	Layout string `json:"layout"`
	Label  string `json:"label"`
	Module string `json:"module"`
	Status string `json:"status"`
}

// HandleSystemPageManifests lists RFC-native page manifests.
// GET /api/v1/system/page-manifests
func (h *Handler) HandleSystemPageManifests(c *gin.Context) {
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}

	manifests, err := store.LoadPageManifests(site)
	if err != nil {
		internalError(c, "loading page manifests", err)
		return
	}

	items := make([]pageManifestListEntry, 0, len(manifests))
	for _, manifest := range manifests {
		if manifest == nil {
			continue
		}
		items = append(items, pageManifestListEntry{
			Name:   manifest.Metadata.Name,
			Route:  manifest.Spec.Route,
			Layout: manifest.Spec.Layout.Type,
			Label:  manifest.Metadata.Name,
			Module: manifest.Metadata.Package,
			Status: manifest.Metadata.Status,
		})
	}

	c.Header("ETag", pageManifestETag(items))
	c.JSON(http.StatusOK, Response{Data: items})
}

// HandleSystemPageManifest returns a single RFC page manifest by name.
// GET /api/v1/system/page-manifests/:name
func (h *Handler) HandleSystemPageManifest(c *gin.Context) {
	name := c.Param("name")
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}

	manifest, err := store.LoadPageManifest(name, site)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Page manifest not found: " + name}})
		return
	}

	c.Header("ETag", pageManifestETag(manifest))
	c.JSON(http.StatusOK, Response{Data: manifest})
}

// HandleSystemPageManifestCreate creates a new RFC page manifest.
// POST /api/v1/system/page-manifests
func (h *Handler) HandleSystemPageManifestCreate(c *gin.Context) {
	var manifest doctype.PageManifest
	if err := c.ShouldBindJSON(&manifest); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid page manifest JSON: " + err.Error()}})
		return
	}

	if err := manifest.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}

	if err := store.SavePageManifest(&manifest, site); err != nil {
		internalError(c, "saving page manifest", err)
		return
	}

	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}
	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Created page manifest "+manifest.Metadata.Name, "Draft", snapshot)
	if err != nil {
		internalError(c, "creating config version", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"manifest":    manifest,
		"version_id":  versionID,
		"version_num": versionNum,
		"status":      "Draft",
	}})
}

// HandleSystemPageManifestUpdate updates an existing RFC page manifest.
// PUT /api/v1/system/page-manifests/:name
func (h *Handler) HandleSystemPageManifestUpdate(c *gin.Context) {
	name := c.Param("name")
	var manifest doctype.PageManifest
	if err := c.ShouldBindJSON(&manifest); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid page manifest JSON: " + err.Error()}})
		return
	}
	manifest.Metadata.Name = name

	if err := manifest.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": err.Error()}})
		return
	}

	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}

	if err := store.SavePageManifest(&manifest, site); err != nil {
		internalError(c, "saving page manifest", err)
		return
	}

	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}
	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Updated page manifest "+manifest.Metadata.Name, "Draft", snapshot)
	if err != nil {
		internalError(c, "creating config version", err)
		return
	}

	c.JSON(http.StatusOK, Response{Data: map[string]any{
		"manifest":    manifest,
		"version_id":  versionID,
		"version_num": versionNum,
		"status":      "Draft",
	}})
}

// HandleSystemPageManifestDelete removes a page manifest by name.
// DELETE /api/v1/system/page-manifests/:name
func (h *Handler) HandleSystemPageManifestDelete(c *gin.Context) {
	name := c.Param("name")
	site := siteName(c)
	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}

	if err := store.DeleteView(name, site); err != nil {
		internalError(c, "deleting page manifest", err)
		return
	}

	reg := h.siteRegistry(c)
	snapshot, err := store.CollectSnapshot(reg, site)
	if err != nil {
		internalError(c, "collecting snapshot", err)
		return
	}
	versionID, versionNum, err := store.CreateConfigVersion(site, currentUser(c), "Deleted page manifest "+name, "Draft", snapshot)
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

// HandlePageManifestByRoute resolves a manifest by runtime route.
// GET /api/v1/page-manifests?route=/orders
func (h *Handler) HandlePageManifestByRoute(c *gin.Context) {
	route := c.Query("route")
	if route == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "route query parameter is required"}})
		return
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}

	store := h.viewStore(c)
	if store == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "page manifest store not available"}})
		return
	}
	manifests, err := store.LoadPageManifests(siteName(c))
	if err != nil {
		internalError(c, "loading page manifests", err)
		return
	}
	for _, manifest := range manifests {
		if manifest != nil && manifest.Spec.Route == route {
			c.JSON(http.StatusOK, Response{Data: manifest})
			return
		}
	}
	c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Page manifest not found for route: " + route}})
}

func pageManifestETag(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
