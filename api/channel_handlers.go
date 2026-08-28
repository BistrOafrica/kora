package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/asenawritescode/kora/api/ai"
	"github.com/asenawritescode/kora/auth"
	"github.com/asenawritescode/kora/doctype"
	"github.com/gin-gonic/gin"
)

type channelSessionIssueRequest struct {
	ClientName      string               `json:"client_name"`
	ConversationKey string               `json:"conversation_key"`
	Provider        string               `json:"provider"`
	SenderAddress   string               `json:"sender_address"`
	Permissions     []doctype.Permission `json:"permissions"`
	TrustedUntil    string               `json:"trusted_until"`
	SensitiveUntil  string               `json:"sensitive_until"`
}

type channelSessionRevokeRequest struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

type channelToolRequest struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
}

func (h *Handler) HandleChannelSessionIssue(c *gin.Context) {
	if c.GetString("auth_type") != "extension" || c.GetString("extension_name") != "kora-cloud-channel" {
		writeError(c, http.StatusForbidden, "auth.extension_required", "extension authentication required", nil)
		return
	}
	var req channelSessionIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "invalid request body", nil)
		return
	}
	if req.ConversationKey == "" || req.Provider == "" || req.SenderAddress == "" || req.TrustedUntil == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "conversation_key, provider, sender_address, and trusted_until are required", nil)
		return
	}
	trustedUntil, err := time.Parse(time.RFC3339, req.TrustedUntil)
	if err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_datetime", "trusted_until must be RFC3339", nil)
		return
	}
	var sensitiveUntil *time.Time
	if strings.TrimSpace(req.SensitiveUntil) != "" {
		parsed, err := time.Parse(time.RFC3339, req.SensitiveUntil)
		if err != nil {
			writeError(c, http.StatusBadRequest, "validation.invalid_datetime", "sensitive_until must be RFC3339", nil)
			return
		}
		sensitiveUntil = &parsed
	}
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)
	clientName := req.ClientName
	if clientName == "" {
		clientName = c.GetString("extension_name")
	}
	token, session, err := auth.CreateChannelSession(h.queryDB(c), auth.ChannelSessionCreateParams{
		Site:            siteNameStr,
		ClientName:      clientName,
		ConversationKey: req.ConversationKey,
		Provider:        req.Provider,
		SenderAddress:   req.SenderAddress,
		Permissions:     req.Permissions,
		TrustedUntil:    trustedUntil,
		SensitiveUntil:  sensitiveUntil,
	})
	if err != nil {
		internalError(c, "issue channel session", err)
		return
	}
	c.JSON(http.StatusCreated, Response{Data: channelSessionIssueResponse{
		SessionID:       session.ID,
		AccessToken:     token,
		TrustedUntil:    session.TrustedUntil,
		SensitiveUntil:  optionalTime(session.SensitiveUntil),
		ConversationKey: session.ConversationKey,
	}})
}

func (h *Handler) HandleChannelSessionRevoke(c *gin.Context) {
	if c.GetString("auth_type") != "extension" || c.GetString("extension_name") != "kora-cloud-channel" {
		writeError(c, http.StatusForbidden, "auth.extension_required", "extension authentication required", nil)
		return
	}
	var req channelSessionRevokeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "session_id is required", map[string]any{"field": "session_id"})
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "revoked_by_client"
	}
	if err := auth.RevokeChannelSession(h.queryDB(c), req.SessionID, reason); err != nil {
		internalError(c, "revoke channel session", err)
		return
	}
	c.JSON(http.StatusOK, Response{Data: channelSessionRevokeResponse{Status: "revoked"}})
}

func (h *Handler) HandleChannelTools(c *gin.Context) {
	if c.GetString("auth_type") != "extension" || c.GetString("extension_name") != "kora-cloud-channel" {
		writeError(c, http.StatusForbidden, "auth.extension_required", "extension authentication required", nil)
		return
	}
	channel := strings.TrimSpace(c.DefaultQuery("channel", "web"))
	catalog := ai.BuildToolCatalog(h.siteRegistry(c))
	filtered := make([]ai.ToolDescriptor, 0, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		if channelAllowed(tool.ChannelAllowlist, channel) {
			filtered = append(filtered, tool)
		}
	}
	version := ai.ToolCatalog{
		Version: catalog.Version,
		Tools:   filtered,
	}
	if match := strings.TrimSpace(c.GetHeader("If-None-Match")); match != "" && match == version.Version {
		c.Status(http.StatusNotModified)
		return
	}
	c.Header("ETag", version.Version)
	c.JSON(http.StatusOK, Response{Data: channelToolsResponse{Version: version.Version, Tools: version.Tools}})
}

func (h *Handler) HandleChannelQuery(c *gin.Context) {
	h.handleChannelTool(c, true)
}

func (h *Handler) HandleChannelMutate(c *gin.Context) {
	h.handleChannelTool(c, false)
}

func (h *Handler) handleChannelTool(c *gin.Context, readOnly bool) {
	if c.GetString("auth_type") != "channel_session" {
		writeError(c, http.StatusForbidden, "auth.authentication_required", "channel session authentication required", nil)
		return
	}
	var req channelToolRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ToolName == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "tool_name is required", map[string]any{"field": "tool_name"})
		return
	}
	safety := ai.BuildToolCatalog(h.siteRegistry(c))
	var descriptor *ai.ToolDescriptor
	for i := range safety.Tools {
		if safety.Tools[i].Name == req.ToolName {
			descriptor = &safety.Tools[i]
			break
		}
	}
	if descriptor == nil {
		writeError(c, http.StatusNotFound, "tool.not_found", "tool not found", nil)
		return
	}
	if docType, operation, ok := permissionTargetForTool(h.siteRegistry(c), req.ToolName); ok {
		if _, forbidden := checkPerm(c, h.siteRegistry(c), docType, operation); forbidden {
			return
		}
	}
	if !channelAllowed(descriptor.ChannelAllowlist, "whatsapp") {
		writeError(c, http.StatusForbidden, "permission.denied", "tool is not allowed on this channel", nil)
		return
	}
	if readOnly && descriptor.SafetyLevel != "safe" {
		writeError(c, http.StatusForbidden, "permission.denied", "tool is not allowed on query endpoint", nil)
		return
	}
	if !readOnly && descriptor.RequiresConfirmation {
		if strings.TrimSpace(c.GetHeader("X-Kora-Confirm")) != "confirmed" {
			writeError(c, http.StatusConflict, "validation.required_confirmation", "confirmation required", nil)
			return
		}
	}
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)
	owner := "channel:" + c.GetString("channel_sender_address")
	result := ai.ExecuteTool(h.siteTx(c), h.siteRegistry(c), req.ToolName, req.Args, owner, siteNameStr)
	status := "success"
	var errorMessage string
	if strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "Unknown tool:") {
		status = "error"
		errorMessage = result
	}
	if err := h.insertChannelAudit(c, req.ToolName, ternary(readOnly, "query", "mutate"), status, summarizeArgs(req.Args), summarizeText(result), errorMessage); err != nil {
		internalError(c, "insert channel audit", err)
		return
	}
	if status == "error" {
		writeError(c, http.StatusBadRequest, "tool.execution_failed", result, nil)
		return
	}
	c.JSON(http.StatusOK, Response{Data: channelToolResponse{Result: result}})
}

func (h *Handler) insertChannelAudit(c *gin.Context, toolName, operationKind, status, requestSummary, responseSummary, errorMessage string) error {
	db := h.queryDB(c)
	if db == nil {
		return sql.ErrConnDone
	}
	siteName, _ := c.Get("site_name")
	siteNameStr, _ := siteName.(string)
	_, err := db.Exec(`INSERT INTO _kora_channel_audit
		(id, site, channel_session_id, conversation_key, provider, sender_address, tool_name, operation_kind, status, request_summary, response_summary, error_message, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		auth.NewAuditID(), siteNameStr, c.GetString("channel_session_id"), c.GetString("channel_conversation_key"),
		"twilio_whatsapp", c.GetString("channel_sender_address"), toolName, operationKind, status, requestSummary, responseSummary, errorMessage, time.Now().UTC(),
	)
	return err
}

func channelAllowed(allowlist []string, channel string) bool {
	for _, item := range allowlist {
		if item == channel {
			return true
		}
	}
	return false
}

func summarizeArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	text := strings.TrimSpace(summarizeText(mustJSON(args)))
	return text
}

func mustJSON(v any) string {
	data, _ := json.Marshal(gin.H{"data": v})
	return string(data)
}

func summarizeText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 500 {
		return text[:500]
	}
	return text
}

func ternary[T any](condition bool, yes, no T) T {
	if condition {
		return yes
	}
	return no
}

func permissionTargetForTool(reg *doctype.Registry, name string) (string, string, bool) {
	for _, item := range []struct {
		suffix    string
		operation string
	}{
		{suffix: "_find", operation: "read"},
		{suffix: "_list", operation: "read"},
		{suffix: "_get", operation: "read"},
		{suffix: "_create", operation: "create"},
		{suffix: "_update", operation: "write"},
		{suffix: "_delete", operation: "delete"},
	} {
		if strings.HasSuffix(name, item.suffix) {
			key := strings.TrimSpace(strings.TrimSuffix(name, item.suffix))
			for _, dt := range reg.All() {
				if sanitizeToolDoctypeName(dt.Name) == key {
					return dt.Name, item.operation, true
				}
			}
			return "", "", false
		}
	}
	return "", "", false
}

func sanitizeToolDoctypeName(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func optionalTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
