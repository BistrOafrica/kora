package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/asenawritescode/kora/webhook"
	"github.com/gin-gonic/gin"
)

// HandleExtensionList returns all extensions for the current site.
func (h *Handler) HandleExtensionList(c *gin.Context) {
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)

	rows, err := h.queryDB(c).Query(
		`SELECT name, site, display_name, description, endpoint_url, is_active, subscriptions, api_permissions,
		 secret_count, consecutive_failures, installed_at, last_delivery_at, last_error
		 FROM _kora_extension WHERE site = ? ORDER BY installed_at DESC`, siteNameStr)
	if err != nil {
		c.JSON(http.StatusOK, Response{Data: []any{}})
		return
	}
	defer rows.Close()

	var extensions []extensionSummary
	for rows.Next() {
		var name, site, displayName, desc, endpointURL, lastErr string
		var subsJSON, permsJSON sql.NullString
		var isActive bool
		var secretCount, consecutiveFailures int
		var installedAt, lastDeliveryAt sql.NullString
		rows.Scan(&name, &site, &displayName, &desc, &endpointURL, &isActive, &subsJSON, &permsJSON,
			&secretCount, &consecutiveFailures, &installedAt, &lastDeliveryAt, &lastErr)
		extensions = append(extensions, extensionSummary{
			Name:                name,
			DisplayName:         displayName,
			Description:         desc,
			EndpointURL:         endpointURL,
			IsActive:            isActive,
			Subscriptions:       subsJSON.String,
			APIPermissions:      permsJSON.String,
			SecretCount:         secretCount,
			ConsecutiveFailures: consecutiveFailures,
			InstalledAt:         installedAt.String,
			LastDeliveryAt:      lastDeliveryAt.String,
			LastError:           lastErr,
		})
	}
	c.JSON(http.StatusOK, Response{Data: extensionListResponse{Extensions: extensions}})
}

// HandleExtensionGet returns a single extension.
func (h *Handler) HandleExtensionGet(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Data: extensionGetResponse{Status: "ok"}})
}

// HandleExtensionCreate registers a new extension.
func (h *Handler) HandleExtensionCreate(c *gin.Context) {
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)

	var req struct {
		Name           string `json:"name"`
		DisplayName    string `json:"display_name"`
		Description    string `json:"description"`
		EndpointURL    string `json:"endpoint_url"`
		Subscriptions  string `json:"subscriptions"`   // JSON array
		APIPermissions string `json:"api_permissions"` // JSON array
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.EndpointURL == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "name and endpoint_url are required", map[string]any{"fields": []string{"name", "endpoint_url"}})
		return
	}

	// Generate signing secret and access token.
	secret, err := webhook.GenerateSecret()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "extension.secret_generation_failed", "Failed to generate secret", nil)
		return
	}
	accessToken, err := generateAccessToken()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "extension.token_generation_failed", "Failed to generate access token", nil)
		return
	}

	// Default empty api_permissions to "[]" — never store null/empty.
	apiPerms := req.APIPermissions
	if apiPerms == "" || apiPerms == "null" {
		apiPerms = "[]"
	}

	db := h.queryDB(c)
	if db == nil {
		writeError(c, http.StatusInternalServerError, "server.database_unavailable", "Database not available", nil)
		return
	}

	_, err = db.Exec(
		`INSERT INTO _kora_extension (name, site, display_name, description, endpoint_url, secret, access_token, subscriptions, api_permissions, installed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(6), NOW(6))`,
		req.Name, siteNameStr, req.DisplayName, req.Description, req.EndpointURL, secret, accessToken,
		req.Subscriptions, apiPerms)
	if err != nil {
		slog.Error("creating extension", "error", err)
		writeError(c, http.StatusInternalServerError, "extension.create_failed", "Failed to create extension", nil)
		return
	}

	slog.Info("extension registered", "name", req.Name, "site", siteNameStr)
	// Return secret and access token — shown once.
	c.JSON(http.StatusCreated, Response{Data: extensionCreatedResponse{
		Name:        req.Name,
		Secret:      secret,
		AccessToken: accessToken,
		Warning:     "Store these credentials securely. They will not be shown again.",
	}})
}

// HandleExtensionUpdate updates an extension.
func (h *Handler) HandleExtensionUpdate(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Data: extensionGetResponse{Status: "ok"}})
}

// HandleExtensionDelete removes an extension.
func (h *Handler) HandleExtensionDelete(c *gin.Context) {
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)
	name := c.Param("name")

	db := h.queryDB(c)
	if db == nil {
		writeError(c, http.StatusNotFound, "extension.not_found", "Not found", nil)
		return
	}
	db.Exec(`DELETE FROM _kora_extension WHERE site = ? AND name = ?`, siteNameStr, name)
	db.Exec(`DELETE FROM _kora_webhook_delivery WHERE extension_name = ?`, name)
	c.JSON(http.StatusOK, Response{Data: extensionDeleteResponse{Status: "deleted"}})
}

// HandleExtensionDeliveries returns the delivery log for an extension.
func (h *Handler) HandleExtensionDeliveries(c *gin.Context) {
	name := c.Param("name")
	db := h.queryDB(c)
	if db == nil {
		c.JSON(http.StatusOK, Response{Data: []any{}})
		return
	}

	rows, err := db.Query(
		`SELECT id, event_id, event_type, endpoint_url, status, attempt, response_status, duration_ms, error_message, created_at
		 FROM _kora_webhook_delivery WHERE extension_name = ? ORDER BY created_at DESC LIMIT 50`, name)
	if err != nil {
		c.JSON(http.StatusOK, Response{Data: []any{}})
		return
	}
	defer rows.Close()

	var deliveries []extensionDelivery
	for rows.Next() {
		var id, eventID, eventType, endpointURL, status, errMsg, createdAt string
		var attempt, respStatus, durationMs int
		rows.Scan(&id, &eventID, &eventType, &endpointURL, &status, &attempt, &respStatus, &durationMs, &errMsg, &createdAt)
		deliveries = append(deliveries, extensionDelivery{
			ID:             id,
			EventID:        eventID,
			EventType:      eventType,
			EndpointURL:    endpointURL,
			Status:         status,
			Attempt:        attempt,
			ResponseStatus: respStatus,
			DurationMs:     durationMs,
			ErrorMessage:   errMsg,
			CreatedAt:      createdAt,
		})
	}
	c.JSON(http.StatusOK, Response{Data: extensionDeliveriesResponse{Deliveries: deliveries}})
}

// HandleExtensionReplay replays a specific delivery or all dead-lettered deliveries.
func (h *Handler) HandleExtensionReplay(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Data: extensionReplayResponse{Status: "replay triggered"}})
}

// HandleExtensionRotateSecret generates a new signing secret for an extension.
func (h *Handler) HandleExtensionRotateSecret(c *gin.Context) {
	name := c.Param("name")
	secret, err := webhook.GenerateSecret()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "extension.secret_generation_failed", "Failed to generate secret", nil)
		return
	}
	db := h.queryDB(c)
	if db == nil {
		writeError(c, http.StatusInternalServerError, "server.database_unavailable", "Database not available", nil)
		return
	}

	// Move current secret to old_secret, set 24h expiry.
	db.Exec(`UPDATE _kora_extension SET old_secret = secret, old_secret_expires_at = ?,
		secret = ?, secret_count = secret_count + 1, updated_at = NOW(6) WHERE name = ?`,
		time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"), secret, name)

	c.JSON(http.StatusOK, Response{Data: extensionRotatedSecretResponse{
		Secret:  secret,
		Warning: "Update your extension with this new secret. Both old and new secrets are valid for 24 hours.",
	}})
}

// queryDB returns the site's database or the handler's default.
func (h *Handler) queryDB(c *gin.Context) *sql.DB {
	if db, ok := c.Get("site_db"); ok {
		if sqlDB, ok := db.(*sql.DB); ok {
			return sqlDB
		}
	}
	return h.TxManager.DB
}

func generateAccessToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
