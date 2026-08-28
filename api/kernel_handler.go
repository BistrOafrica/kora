package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/kernel"
	"github.com/asenawritescode/kora/outbox"
	"github.com/gin-gonic/gin"
)

// kernelRequest is the wire shape for POST /api/v1/kernel/:command.
type kernelRequest struct {
	IdempotencyKey  string          `json:"idempotency_key"`
	CorrelationID   string          `json:"correlation_id"`
	CausationID     string          `json:"causation_id"`
	ExpectedVersion string          `json:"expected_version"`
	Payload         json.RawMessage `json:"payload"`
}

// sourceForAuthType maps the gin auth context onto a kernel Source. The
// kernel treats every source identically; the mapping is audit metadata.
func sourceForAuthType(authType string) kernel.Source {
	switch authType {
	case "extension":
		return kernel.SourceIntegration
	case "channel_session":
		return kernel.SourceUI
	default:
		return kernel.SourceHTTP
	}
}

// siteOutboxWriter resolves the current site's outbox writer from request
// context, falling back to the site map on the handler.
func (h *Handler) siteOutboxWriter(c *gin.Context) outbox.Writer {
	siteName, _ := c.Get("site_name")
	if s, ok := siteName.(string); ok && h.SiteOutboxes != nil {
		if w, exists := h.SiteOutboxes[s]; exists {
			return w
		}
	}
	return nil
}

// HandleKernelRegistry lists config-defined command resources available on
// this runtime (KERNEL-008 introspection): name, version, permission
// operation, input record, and touched records with their required
// operations. MCP/AI/SDK catalogs derive from this surface.
func (h *Handler) HandleKernelRegistry(c *gin.Context) {
	siteName, _ := c.Get("site_name")
	site, _ := siteName.(string)
	commands := h.KernelCommands
	if commands == nil {
		commands = kernel.NewCommandRegistry()
	}
	type entry struct {
		Name       string   `json:"name"`
		Version    int      `json:"version"`
		Permission string   `json:"permission"`
		Input      string   `json:"input_record"`
		Touched    []string `json:"touched_records"`
	}
	out := []entry{}
	for _, def := range commands.List() {
		touched := def.TouchedRecords()
		if touched == nil {
			touched = []string{}
		}
		out = append(out, entry{
			Name:       def.FullName(),
			Version:    def.Version,
			Permission: def.PermOperation(),
			Input:      def.Input.Record,
			Touched:    touched,
		})
	}
	c.JSON(http.StatusOK, gin.H{"site": site, "commands": out})
}

// HandleKernelOperation routes a canonical command through the operation
// kernel (first vertical slice: record.create, record.update). All sources
// share this path; there is no per-adapter mutation logic here.
func (h *Handler) HandleKernelOperation(c *gin.Context) {
	siteName, _ := c.Get("site_name")
	site, _ := siteName.(string)
	if site == "" {
		badRequestError(c, "request.no_tenant_context", "no tenant context", nil)
		return
	}
	dbVal, _ := c.Get("site_db")
	sqlDB, ok := dbVal.(*sql.DB)
	if !ok || sqlDB == nil {
		internalError(c, "site database unavailable", nil)
		return
	}
	reg := h.siteRegistry(c)

	var req kernelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "validation.invalid_json", "Invalid request format", nil)
		return
	}

	commandName := c.Param("command")
	user := c.GetString("user")
	if user == "" {
		user = "system"
	}
	opCtx := kernel.OperationContext{
		Site:           site,
		User:           user,
		Roles:          c.GetStringSlice("user_roles"),
		Source:         sourceForAuthType(c.GetString("auth_type")),
		CorrelationID:  req.CorrelationID,
		CausationID:    req.CausationID,
		ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	}
	opCtx.Actor = contract.ActorContext{
		PrincipalID:     user,
		PrincipalType:   contract.PrincipalHuman,
		SubjectUserID:   user,
		Site:            site,
		Roles:           opCtx.Roles,
		AuthenticatedAt: time.Now(),
	}

	k := kernel.New(h.TxManager.Dialect, h.siteOutboxWriter(c))
	k.Commands = h.KernelCommands
	result, cerr := k.Execute(c.Request.Context(), sqlDB, reg, kernel.Operation{
		Context:  opCtx,
		Command:  commandName,
		Payload:  req.Payload,
		Deadline: time.Now().Add(30 * time.Second),
	})
	if result.Replayed {
		c.Header("X-Kora-Replay", "true")
	}
	if cerr != nil {
		status := http.StatusInternalServerError
		switch cerr.Type {
		case contract.CodePermissionDenied, contract.CodeUnauthenticated:
			status = http.StatusForbidden
			if cerr.Type == contract.CodeUnauthenticated {
				status = http.StatusUnauthorized
			}
		case contract.CodeValidationFailed, contract.CodeNotFound, contract.CodeIdempotencyKeyReused:
			status = http.StatusBadRequest
		case contract.CodeConflict:
			status = http.StatusConflict
		case contract.CodeDeadlineExceeded:
			status = http.StatusGatewayTimeout
		}
		writeError(c, status, "kernel."+string(cerr.Type), cerr.Message, nil)
		return
	}

	meta := Meta{DocType: commandName}
	c.JSON(http.StatusOK, Response{Data: json.RawMessage(result.Data), Meta: &meta})
}
