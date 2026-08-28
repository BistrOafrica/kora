package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/asenawritescode/kora/auth"
	sqlDialect "github.com/asenawritescode/kora/db"
	"github.com/asenawritescode/kora/net"
	"github.com/asenawritescode/kora/site"
)

// onboardRateLimiter tracks IP → count for self-service site creation.
var (
	onboardLimiter   = make(map[string]int)
	onboardLimiterMu sync.Mutex
	onboardLimitMax  = 3
)

// ConsoleHandler holds dependencies for console API endpoints.
type ConsoleHandler struct {
	SystemGuard        *auth.SystemGuard
	SiteRouter         *net.SiteRouter
	ProvisioningStore  *site.OnboardingStore
	AllowOnboarding    bool
	queuedJobsMu       sync.Mutex
	queuedJobs         map[string]bool
	PlatformDBType     string
	PlatformDBHost     string
	PlatformDBPort     int
	PlatformDBUser     string
	PlatformDBPassword string
	PlatformDB         *sql.DB // Existing platform DB connection (for LibSQL reuse)
}

// NewConsoleHandler creates a console API handler.
func NewConsoleHandler(guard *auth.SystemGuard, sr *net.SiteRouter, dbType, dbHost, dbUser, dbPassword string, dbPort int, platformDB *sql.DB, allowOnboarding bool) *ConsoleHandler {
	return &ConsoleHandler{
		SystemGuard:        guard,
		SiteRouter:         sr,
		ProvisioningStore:  site.NewOnboardingStore(platformDB),
		AllowOnboarding:    allowOnboarding,
		queuedJobs:         make(map[string]bool),
		PlatformDBType:     dbType,
		PlatformDBHost:     dbHost,
		PlatformDBPort:     dbPort,
		PlatformDBUser:     dbUser,
		PlatformDBPassword: dbPassword,
		PlatformDB:         platformDB,
	}
}

// Start begins the background rate limiter reset loop for self-service onboarding.
func (h *ConsoleHandler) Start() {
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			onboardLimiterMu.Lock()
			onboardLimiter = make(map[string]int)
			onboardLimiterMu.Unlock()
		}
	}()
	if h.ProvisioningStore != nil {
		if err := h.ProvisioningStore.Bootstrap(); err != nil {
			slog.Error("provisioning store bootstrap failed", "error", err)
		}
	}
}

func (h *ConsoleHandler) scheduleOnboard(job site.OnboardingJob, req onboardRequest) {
	h.queuedJobsMu.Lock()
	if h.queuedJobs[job.ID] {
		h.queuedJobsMu.Unlock()
		return
	}
	h.queuedJobs[job.ID] = true
	h.queuedJobsMu.Unlock()

	go func() {
		defer func() {
			h.queuedJobsMu.Lock()
			delete(h.queuedJobs, job.ID)
			h.queuedJobsMu.Unlock()
		}()
		h.processOnboardJob(job, req)
	}()
}

type onboardRequest struct {
	Hostname      string `json:"hostname"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
	AdminFullName string `json:"admin_full_name"`
	PlatformType  string `json:"platform_type"`
	PlatformHost  string `json:"platform_host"`
	PlatformPort  int    `json:"platform_port"`
	PlatformUser  string `json:"platform_user"`
	PlatformPass  string `json:"platform_password"`
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// HandleLogin authenticates a console super-admin.
// POST /api/console/login
func (h *ConsoleHandler) HandleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", nil)
		return
	}

	valid, needsChange := h.SystemGuard.ValidateWithChangeCheck(req.Email, req.Password)
	if !valid {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: ErrorBody{
			Code:    "auth.invalid_credentials",
			Message: "Invalid credentials",
		}})
		return
	}

	token := h.SystemGuard.CreateSession(req.Email)
	c.JSON(http.StatusOK, Response{Data: consoleLoginResponse{
		Token:               token,
		Email:               req.Email,
		NeedsPasswordChange: needsChange,
	}})
}

// HandleChangePassword forces a password change (required on first login with default creds).
// POST /api/console/change-password
func (h *ConsoleHandler) HandleChangePassword(c *gin.Context) {
	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	if !h.SystemGuard.ValidateSessionBool(token) {
		writeError(c, http.StatusUnauthorized, "auth.session_invalid", "Invalid session", nil)
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NewPassword == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "New password required", map[string]any{"field": "new_password"})
		return
	}

	h.SystemGuard.UpdatePassword(req.NewPassword)
	c.JSON(http.StatusOK, Response{Data: consoleMessageResponse{Message: "Password changed"}})
}

// ---------------------------------------------------------------------------
// Auth middleware for console API routes
// ---------------------------------------------------------------------------

// RequireConsoleAuth is middleware that validates the console session.
// Accepts Authorization: Bearer <token> header OR kora_console_sid cookie.
func (h *ConsoleHandler) RequireConsoleAuth(c *gin.Context) {
	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	if token == "" {
		// Fallback: check the console session cookie.
		if sid, err := c.Cookie("kora_console_sid"); err == nil && sid != "" {
			token = sid
		}
	}
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: ErrorBody{
			Code:    "auth.authentication_required",
			Message: "Authentication required",
		}})
		return
	}
	if !h.SystemGuard.ValidateSessionBool(token) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: ErrorBody{
			Code:    "auth.session_invalid",
			Message: "Invalid or expired session",
		}})
		return
	}
	c.Next()
}

// ---------------------------------------------------------------------------
// Site Management
// ---------------------------------------------------------------------------

// HandleListSites returns all loaded sites with status.
// Falls back to querying the database directly when no sites are loaded (e.g. after
// container redeploy where site_config.yaml files were lost but DB data persists).
// GET /api/console/sites
func (h *ConsoleHandler) HandleListSites(c *gin.Context) {
	sites := h.SiteRouter.AllSites()

	// If no sites in memory, try the database — sites survive redeploys there.
	if len(sites) == 0 && h.PlatformDB != nil {
		if dbSites, err := site.DiscoverSitesFromDB(h.PlatformDB); err == nil {
			for _, info := range dbSites {
				sites = append(sites, &net.LoadedSite{
					Name:   info.Name,
					Config: net.SiteRouterConfig{Hostname: info.Name, Domains: info.Domains},
					DB:     h.PlatformDB,
				})
			}
		}
	}

	type SiteEntry struct {
		Name     string   `json:"name"`
		Domains  []string `json:"domains"`
		DocTypes int      `json:"doctypes"`
		Status   string   `json:"status"`
	}
	var result []SiteEntry
	for _, s := range sites {
		status := "active"
		if s.DB != nil {
			if err := s.DB.Ping(); err != nil {
				status = "error"
			}
		} else {
			status = "unknown"
		}
		result = append(result, SiteEntry{
			Name:     s.Name,
			Domains:  s.Config.Domains,
			DocTypes: len(s.Registry.All()),
			Status:   status,
		})
	}
	if result == nil {
		result = []SiteEntry{} // return empty array, not null
	}
	c.JSON(http.StatusOK, Response{Data: result})
}

// HandleCreateSite creates a new site: database, config, bootstrap, admin user.
// POST /api/console/sites
// Only hostname, admin_email, and admin_password are required.
// DB fields are optional — platform defaults from env vars are used when empty.
func (h *ConsoleHandler) HandleCreateSite(c *gin.Context) {
	var req struct {
		Hostname      string `json:"hostname"`
		DBType        string `json:"db_type"`
		DBHost        string `json:"db_host"`
		DBPort        int    `json:"db_port"`
		DBName        string `json:"db_name"`
		DBUser        string `json:"db_user"`
		DBPassword    string `json:"db_password"`
		Domains       string `json:"domains"` // comma-separated extra domains
		AdminEmail    string `json:"admin_email"`
		AdminPassword string `json:"admin_password"`
		AdminFullName string `json:"admin_full_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", map[string]any{"error": err.Error()})
		return
	}
	if req.Hostname == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "hostname, admin_email, and admin_password are required", map[string]any{"fields": []string{"hostname", "admin_email", "admin_password"}})
		return
	}

	slog.Info("creating site via console", "hostname", req.Hostname)

	// Resolve platform DB credentials: handler fields (from common config) → env vars.
	platformType := h.PlatformDBType
	if platformType == "" {
		platformType = os.Getenv("KORA_DB_TYPE")
	}
	if platformType == "" {
		platformType = "mysql"
	}
	platformHost := h.PlatformDBHost
	if platformHost == "" {
		platformHost = os.Getenv("KORA_DB_HOST")
	}
	platformPort := h.PlatformDBPort
	if platformPort == 0 {
		platformPort = envConsoleInt("KORA_DB_PORT")
	}
	platformUser := h.PlatformDBUser
	if platformUser == "" {
		platformUser = os.Getenv("KORA_DB_USER")
	}
	platformPass := h.PlatformDBPassword
	if platformPass == "" {
		platformPass = os.Getenv("KORA_DB_PASSWORD")
	}

	// Parse comma-separated extra domains.
	var extraDomains []string
	if req.Domains != "" {
		for _, d := range strings.Split(req.Domains, ",") {
			d = strings.TrimSpace(d)
			if d != "" && d != req.Hostname {
				extraDomains = append(extraDomains, d)
			}
		}
	}

	result, err := site.CreateSite(site.CreateSiteInput{
		Hostname:           req.Hostname,
		DBType:             req.DBType,
		DBHost:             req.DBHost,
		DBPort:             req.DBPort,
		DBName:             req.DBName,
		DBUser:             req.DBUser,
		DBPassword:         req.DBPassword,
		AdminEmail:         req.AdminEmail,
		AdminPassword:      req.AdminPassword,
		AdminFullName:      req.AdminFullName,
		ExtraDomains:       extraDomains,
		PlatformDBType:     platformType,
		PlatformDBHost:     platformHost,
		PlatformDBPort:     platformPort,
		PlatformDBUser:     platformUser,
		PlatformDBPassword: platformPass,
		PlatformDBDSN:      os.Getenv("DB_DSN"),
		PlatformDB:         h.PlatformDB,
	})
	if err != nil {
		slog.Error("creating site failed", "hostname", req.Hostname, "error", err)
		errMsg := err.Error()
		// Map known errors to user-friendly messages.
		switch {
		case strings.Contains(errMsg, "connection refused"):
			errMsg = "Cannot connect to MySQL server. Is MySQL running?"
		case strings.Contains(errMsg, "Access denied"):
			errMsg = "Invalid database credentials. Check your DB user and password."
		case strings.Contains(errMsg, "Unknown database"):
			errMsg = "Cannot access the database server. Check your DB host and port."
		default:
			errMsg = "Failed to create site: " + errMsg
		}
		writeError(c, http.StatusInternalServerError, "site.create_failed", errMsg, nil)
		return
	}

	// Hot-add site to the running router.
	// Keep routing domains explicit. Do not auto-add the current request host here:
	// path-based access should work without pretending the app host is a tenant domain.
	domains := []string{req.Hostname}
	domains = append(domains, extraDomains...)
	loaded := &net.LoadedSite{
		Name: req.Hostname,
		Config: net.SiteRouterConfig{
			Hostname: req.Hostname,
			Domains:  domains,
		},
		DB:       result.DB,
		Registry: result.Registry,
	}
	h.SiteRouter.AddSite(loaded)

	slog.Info("site created via console", "hostname", req.Hostname, "db_name", result.Config.DBName)
	c.JSON(http.StatusCreated, Response{Data: consoleSiteCreateResponse{
		Hostname: req.Hostname,
		DBName:   result.Config.DBName,
		Status:   "active",
		Admin:    req.AdminEmail,
	}})
}

// HandleOnboard creates a site via public self-service (no console auth).
// POST /api/console/sites/onboard
// Rate limited: 3 per hour per IP.
func (h *ConsoleHandler) HandleOnboard(c *gin.Context) {
	if !h.AllowOnboarding {
		writeError(c, http.StatusNotFound, "feature.disabled", "Public onboarding is disabled", nil)
		return
	}
	var req onboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", map[string]any{"error": err.Error()})
		return
	}
	if req.Hostname == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "hostname, admin_email, and admin_password are required", map[string]any{"fields": []string{"hostname", "admin_email", "admin_password"}})
		return
	}
	if len(req.AdminPassword) < 8 {
		writeError(c, http.StatusBadRequest, "validation.password_too_short", "Password must be at least 8 characters", nil)
		return
	}

	// Rate limit by client IP.
	ip := c.ClientIP()
	onboardLimiterMu.Lock()
	count := onboardLimiter[ip]
	if count >= onboardLimitMax {
		onboardLimiterMu.Unlock()
		writeError(c, http.StatusTooManyRequests, "rate_limit.exceeded", "Too many requests. Please try again later.", nil)
		return
	}
	onboardLimiter[ip] = count + 1
	onboardLimiterMu.Unlock()

	// Check if hostname is already taken.
	var existing int
	if h.PlatformDB != nil {
		h.PlatformDB.QueryRow("SELECT COUNT(*) FROM _kora_config_version WHERE site = ?", req.Hostname).Scan(&existing)
	}
	if existing > 0 {
		writeError(c, http.StatusConflict, "site.already_exists", "This site name is already taken. Try another.", nil)
		return
	}

	opID := "onboard:" + req.Hostname
	idempotencyKey := req.Hostname + ":" + req.AdminEmail
	fingerprint := req.Hostname + "|" + req.AdminEmail

	if h.ProvisioningStore != nil {
		if err := h.ProvisioningStore.Bootstrap(); err != nil {
			slog.Error("provisioning store bootstrap failed", "error", err)
			writeError(c, http.StatusInternalServerError, "server.store_unavailable", "Failed to initialize provisioning store", nil)
			return
		}
		if existingJob, err := h.ProvisioningStore.LoadJob(req.Hostname, req.Hostname, opID, idempotencyKey, fingerprint); err == nil && existingJob != nil {
			if existingJob.State == site.OnboardingActive {
				c.JSON(http.StatusCreated, Response{Data: consoleSiteCreateResponse{
					Hostname:     req.Hostname,
					WorkspaceURL: "/s/" + req.Hostname + "/workspace",
					AdminEmail:   req.AdminEmail,
					Status:       "active",
				}})
				return
			}
		}
	}

	job := site.OnboardingJob{
		ID:               "prov-" + req.Hostname,
		Site:             req.Hostname,
		Resource:         req.Hostname,
		State:            site.OnboardingRequested,
		OperationID:      opID,
		IdempotencyKey:   idempotencyKey,
		InputFingerprint: fingerprint,
		Attempt:          1,
		CreatedAt:        time.Now().UTC(),
	}
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(site.OnboardingCheckpoint{JobID: job.ID, Stage: "requested", Completed: true, RecordedAt: time.Now().UTC()})
	}

	h.scheduleOnboard(job, req)
	c.JSON(http.StatusAccepted, Response{Data: map[string]any{
		"job_id":        job.ID,
		"hostname":      req.Hostname,
		"admin_email":   req.AdminEmail,
		"status":        "requested",
		"workspace_url": "/s/" + req.Hostname + "/workspace",
	}})
}

// HandleOnboardStatus returns the current state of a provisioning job.
// GET /api/console/sites/onboard/:job_id
func (h *ConsoleHandler) HandleOnboardStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "job_id required", map[string]any{"field": "job_id"})
		return
	}
	if h.ProvisioningStore == nil {
		writeError(c, http.StatusNotFound, "server.store_unavailable", "Provisioning store unavailable", nil)
		return
	}
	job, err := h.ProvisioningStore.LoadJobByID(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "provisioning.job_not_found", "Provisioning job not found", nil)
			return
		}
		slog.Error("loading provisioning job failed", "job_id", jobID, "error", err)
		writeError(c, http.StatusInternalServerError, "provisioning.load_failed", "Failed to load provisioning job", nil)
		return
	}
	c.JSON(http.StatusOK, Response{Data: consoleProvisioningStatusResponse{
		JobID:            job.ID,
		Site:             job.Site,
		State:            string(job.State),
		Attempt:          job.Attempt,
		OperationID:      job.OperationID,
		IdempotencyKey:   job.IdempotencyKey,
		InputFingerprint: job.InputFingerprint,
		OutputID:         job.OutputID,
		LastError:        job.LastError,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
		WorkspaceURL:     "/s/" + job.Site + "/workspace",
	}})
}

// HandleOnboardJobs lists provisioning jobs for the console.
// GET /api/console/sites/onboard
func (h *ConsoleHandler) HandleOnboardJobs(c *gin.Context) {
	if h.ProvisioningStore == nil {
		c.JSON(http.StatusOK, Response{Data: consoleProvisioningListResponse{Jobs: []consoleProvisioningStatusResponse{}}})
		return
	}
	jobs, err := h.ProvisioningStore.ListJobs()
	if err != nil {
		slog.Error("listing provisioning jobs failed", "error", err)
		writeError(c, http.StatusInternalServerError, "provisioning.list_failed", "Failed to list provisioning jobs", nil)
		return
	}
	out := make([]consoleProvisioningStatusResponse, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, consoleProvisioningStatusResponse{
			JobID:            job.ID,
			Site:             job.Site,
			State:            string(job.State),
			Attempt:          job.Attempt,
			OperationID:      job.OperationID,
			IdempotencyKey:   job.IdempotencyKey,
			InputFingerprint: job.InputFingerprint,
			OutputID:         job.OutputID,
			LastError:        job.LastError,
			CreatedAt:        job.CreatedAt,
			UpdatedAt:        job.UpdatedAt,
			WorkspaceURL:     "/s/" + job.Site + "/workspace",
		})
	}
	c.JSON(http.StatusOK, Response{Data: consoleProvisioningListResponse{Jobs: out}})
}

func (h *ConsoleHandler) processOnboardJob(job site.OnboardingJob, req onboardRequest) {
	adminFullName := req.AdminFullName
	if adminFullName == "" {
		adminFullName = strings.Split(req.AdminEmail, "@")[0]
	}
	platformType := req.PlatformType
	if platformType == "" {
		platformType = h.PlatformDBType
	}
	if platformType == "" {
		platformType = os.Getenv("KORA_DB_TYPE")
	}
	if platformType == "" {
		platformType = "mysql"
	}
	platformHost := req.PlatformHost
	if platformHost == "" {
		platformHost = h.PlatformDBHost
	}
	if platformHost == "" {
		platformHost = os.Getenv("KORA_DB_HOST")
	}
	platformPort := req.PlatformPort
	if platformPort == 0 {
		platformPort = h.PlatformDBPort
	}
	if platformPort == 0 {
		platformPort = envConsoleInt("KORA_DB_PORT")
	}
	platformUser := req.PlatformUser
	if platformUser == "" {
		platformUser = h.PlatformDBUser
	}
	if platformUser == "" {
		platformUser = os.Getenv("KORA_DB_USER")
	}
	platformPass := req.PlatformPass
	if platformPass == "" {
		platformPass = h.PlatformDBPassword
	}
	if platformPass == "" {
		platformPass = os.Getenv("KORA_DB_PASSWORD")
	}

	job.State = site.OnboardingValidating
	job.Attempt++
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(site.OnboardingCheckpoint{JobID: job.ID, Stage: "validated", Completed: true, RecordedAt: time.Now().UTC()})
	}

	result, err := site.CreateSite(site.CreateSiteInput{
		Hostname:           req.Hostname,
		AdminEmail:         req.AdminEmail,
		AdminPassword:      req.AdminPassword,
		AdminFullName:      adminFullName,
		PlatformDBType:     platformType,
		PlatformDBHost:     platformHost,
		PlatformDBPort:     platformPort,
		PlatformDBUser:     platformUser,
		PlatformDBPassword: platformPass,
		PlatformDBDSN:      os.Getenv("DB_DSN"),
		PlatformDB:         h.PlatformDB,
	})
	if err != nil {
		job.State = site.OnboardingFailed
		job.LastError = err.Error()
		job.UpdatedAt = time.Now().UTC()
		if h.ProvisioningStore != nil {
			_ = h.ProvisioningStore.UpsertJob(job)
		}
		slog.Error("self-service site creation failed", "hostname", req.Hostname, "error", err)
		return
	}

	job.State = site.OnboardingProvisioning
	job.OutputID = result.Config.DBName
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(site.OnboardingCheckpoint{JobID: job.ID, Stage: "provisioned", Completed: true, RecordedAt: time.Now().UTC()})
	}

	domains := []string{req.Hostname}
	loaded := &net.LoadedSite{
		Name: req.Hostname,
		Config: net.SiteRouterConfig{
			Hostname: req.Hostname,
			Domains:  domains,
		},
		DB:       result.DB,
		Registry: result.Registry,
	}
	h.SiteRouter.AddSite(loaded)

	job.State = site.OnboardingActive
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(site.OnboardingCheckpoint{JobID: job.ID, Stage: "active", Completed: true, RecordedAt: time.Now().UTC()})
	}
	slog.Info("site created via self-service onboarding", "hostname", req.Hostname)
}

// HandleUpdateSite updates site metadata (domains).
// PUT /api/console/sites/:name
func (h *ConsoleHandler) HandleUpdateSite(c *gin.Context) {
	siteName := c.Param("name")
	if siteName == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "site name required", map[string]any{"field": "name"})
		return
	}

	var req struct {
		Domains []string `json:"domains"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", nil)
		return
	}

	site := h.SiteRouter.SiteByName(siteName)
	if site == nil {
		writeError(c, http.StatusNotFound, "site.not_found", "Site not found", map[string]any{"name": siteName})
		return
	}

	// Update domains in the in-memory router.
	site.Config.Domains = req.Domains
	if len(site.Config.Domains) == 0 {
		site.Config.Domains = []string{site.Config.Hostname}
	}

	// Persist to DB if platform DB is available.
	if h.PlatformDB != nil {
		domainsJSON, _ := json.Marshal(req.Domains)
		h.PlatformDB.Exec(
			"UPDATE _kora_config_version SET config = ? WHERE site = ? AND status = 'Active'",
			fmt.Sprintf(`{"domains": %s}`, string(domainsJSON)), siteName,
		)
	}

	slog.Info("site updated via console", "hostname", siteName, "domains", req.Domains)
	c.JSON(http.StatusOK, Response{Data: consoleSiteUpdateResponse{
		Hostname: siteName,
		Domains:  site.Config.Domains,
	}})
}

// HandleDeleteSite deletes a site and all its data.
// DELETE /api/console/sites/:name
// Requires confirmation: {"confirm": "<hostname>"}
func (h *ConsoleHandler) HandleDeleteSite(c *gin.Context) {
	siteName := c.Param("name")
	if siteName == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "site name required", map[string]any{"field": "name"})
		return
	}

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", nil)
		return
	}
	if req.Confirm != siteName {
		writeError(c, http.StatusBadRequest, "validation.required_confirmation", "Type the site hostname to confirm deletion.", nil)
		return
	}

	loaded := h.SiteRouter.SiteByName(siteName)
	if loaded == nil {
		writeError(c, http.StatusNotFound, "site.not_found", "Site not found", map[string]any{"name": siteName})
		return
	}

	// Derive DB name from hostname.
	dbName := strings.ReplaceAll(siteName, ".", "_")

	slog.Info("deleting site via console", "hostname", siteName)

	if err := site.DeleteSite(site.DeleteSiteInput{
		DB:             loaded.DB,
		Dialect:        sqlDialect.Resolve(h.PlatformDBType),
		Hostname:       siteName,
		PlatformDB:     h.PlatformDB,
		PlatformDBType: h.PlatformDBType,
		DBType:         h.PlatformDBType,
		DBName:         dbName,
		DBHost:         h.PlatformDBHost,
		DBPort:         h.PlatformDBPort,
		DBUser:         h.PlatformDBUser,
		DBPassword:     h.PlatformDBPassword,
	}); err != nil {
		slog.Error("deleting site failed", "hostname", siteName, "error", err)
		writeError(c, http.StatusInternalServerError, "site.delete_failed", "Failed to delete site", map[string]any{"error": err.Error()})
		return
	}

	// Remove from the in-memory router.
	h.SiteRouter.RemoveSite(siteName)

	slog.Info("site deleted via console", "hostname", siteName)
	c.JSON(http.StatusOK, Response{Data: consoleSiteDeleteResponse{
		Hostname: siteName,
		Deleted:  true,
	}})
}

// HandleResetSitePassword resets a site user's password.
// POST /api/console/sites/:name/reset-password
func (h *ConsoleHandler) HandleResetSitePassword(c *gin.Context) {
	siteName := c.Param("name")
	if siteName == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "site name required", map[string]any{"field": "name"})
		return
	}

	var req struct {
		Email       string `json:"email"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "validation.invalid_json", "Invalid request", nil)
		return
	}
	if req.Email == "" || req.NewPassword == "" {
		writeError(c, http.StatusBadRequest, "validation.required_field", "email and new_password are required", map[string]any{"fields": []string{"email", "new_password"}})
		return
	}

	loaded := h.SiteRouter.SiteByName(siteName)
	if loaded == nil {
		writeError(c, http.StatusNotFound, "site.not_found", "Site not found", map[string]any{"name": siteName})
		return
	}

	// Hash the new password.
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("hashing password failed", "error", err)
		writeError(c, http.StatusInternalServerError, "auth.password_hash_failed", "Failed to hash password.", nil)
		return
	}

	// Update the user's password in the site's database.
	result, err := loaded.DB.Exec(
		"UPDATE _kora_user SET password_hash = ?, modified = CURRENT_TIMESTAMP WHERE email = ?",
		passwordHash, req.Email,
	)
	if err != nil {
		slog.Error("resetting site password failed", "site", siteName, "email", req.Email, "error", err)
		writeError(c, http.StatusInternalServerError, "site.password_update_failed", "Failed to update password", map[string]any{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeError(c, http.StatusNotFound, "user.not_found", "No user found with email", map[string]any{"email": req.Email})
		return
	}

	// Invalidate all sessions for this user.
	loaded.DB.Exec("DELETE FROM _kora_session WHERE user = (SELECT name FROM _kora_user WHERE email = ?)", req.Email)

	slog.Info("site user password reset via console", "site", siteName, "email", req.Email)
	c.JSON(http.StatusOK, Response{Data: consoleMessageResponse{Message: "Password reset successfully. All existing sessions have been invalidated."}})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// envConsoleInt reads an integer env var, returning 0 if empty or unparseable.
func envConsoleInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n := 0
	for _, c := range v {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
