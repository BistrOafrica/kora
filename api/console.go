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
	"github.com/asenawritescode/kora/cloud"
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
	ProvisioningStore  *cloud.ProvisioningStore
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
func NewConsoleHandler(guard *auth.SystemGuard, sr *net.SiteRouter, dbType, dbHost, dbUser, dbPassword string, dbPort int, platformDB *sql.DB) *ConsoleHandler {
	return &ConsoleHandler{
		SystemGuard:        guard,
		SiteRouter:         sr,
		ProvisioningStore:  cloud.NewProvisioningStore(platformDB),
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

func (h *ConsoleHandler) scheduleOnboard(job cloud.ProvisioningJob, req onboardRequest) {
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request"}})
		return
	}

	valid, needsChange := h.SystemGuard.ValidateWithChangeCheck(req.Email, req.Password)
	if !valid {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: map[string]string{"message": "Invalid credentials"}})
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
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: map[string]string{"message": "Invalid session"}})
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "New password required"}})
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
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: map[string]string{"message": "Authentication required"}})
		return
	}
	if !h.SystemGuard.ValidateSessionBool(token) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: map[string]string{"message": "Invalid or expired session"}})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request: " + err.Error()}})
		return
	}
	if req.Hostname == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "hostname, admin_email, and admin_password are required"}})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": errMsg}})
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
	var req onboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request: " + err.Error()}})
		return
	}
	if req.Hostname == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "hostname, admin_email, and admin_password are required"}})
		return
	}
	if len(req.AdminPassword) < 8 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Password must be at least 8 characters"}})
		return
	}

	// Rate limit by client IP.
	ip := c.ClientIP()
	onboardLimiterMu.Lock()
	count := onboardLimiter[ip]
	if count >= onboardLimitMax {
		onboardLimiterMu.Unlock()
		c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: map[string]string{"message": "Too many requests. Please try again later."}})
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
		c.JSON(http.StatusConflict, ErrorResponse{Error: map[string]string{"message": "This site name is already taken. Try another."}})
		return
	}

	opID := "onboard:" + req.Hostname
	idempotencyKey := req.Hostname + ":" + req.AdminEmail
	fingerprint := req.Hostname + "|" + req.AdminEmail

	if h.ProvisioningStore != nil {
		if err := h.ProvisioningStore.Bootstrap(); err != nil {
			slog.Error("provisioning store bootstrap failed", "error", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to initialize provisioning store"}})
			return
		}
		if existingJob, err := h.ProvisioningStore.LoadJob(req.Hostname, req.Hostname, opID, idempotencyKey, fingerprint); err == nil && existingJob != nil {
			if existingJob.State == cloud.ProvisioningActive {
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

	job := cloud.ProvisioningJob{
		ID:               "prov-" + req.Hostname,
		Site:             req.Hostname,
		Resource:         req.Hostname,
		State:            cloud.ProvisioningRequested,
		OperationID:      opID,
		IdempotencyKey:   idempotencyKey,
		InputFingerprint: fingerprint,
		Attempt:          1,
		CreatedAt:        time.Now().UTC(),
	}
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(cloud.ProvisioningCheckpoint{JobID: job.ID, Stage: "requested", Completed: true, RecordedAt: time.Now().UTC()})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "job_id required"}})
		return
	}
	if h.ProvisioningStore == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Provisioning store unavailable"}})
		return
	}
	job, err := h.ProvisioningStore.LoadJobByID(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Provisioning job not found"}})
			return
		}
		slog.Error("loading provisioning job failed", "job_id", jobID, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to load provisioning job"}})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to list provisioning jobs"}})
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

func (h *ConsoleHandler) processOnboardJob(job cloud.ProvisioningJob, req onboardRequest) {
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

	job.State = cloud.ProvisioningValidating
	job.Attempt++
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(cloud.ProvisioningCheckpoint{JobID: job.ID, Stage: "validated", Completed: true, RecordedAt: time.Now().UTC()})
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
		job.State = cloud.ProvisioningFailed
		job.LastError = err.Error()
		job.UpdatedAt = time.Now().UTC()
		if h.ProvisioningStore != nil {
			_ = h.ProvisioningStore.UpsertJob(job)
		}
		slog.Error("self-service site creation failed", "hostname", req.Hostname, "error", err)
		return
	}

	job.State = cloud.ProvisioningProvisioning
	job.OutputID = result.Config.DBName
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(cloud.ProvisioningCheckpoint{JobID: job.ID, Stage: "provisioned", Completed: true, RecordedAt: time.Now().UTC()})
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

	job.State = cloud.ProvisioningActive
	job.UpdatedAt = time.Now().UTC()
	if h.ProvisioningStore != nil {
		_ = h.ProvisioningStore.UpsertJob(job)
		_ = h.ProvisioningStore.UpsertCheckpoint(cloud.ProvisioningCheckpoint{JobID: job.ID, Stage: "active", Completed: true, RecordedAt: time.Now().UTC()})
	}
	slog.Info("site created via self-service onboarding", "hostname", req.Hostname)
}

// HandleUpdateSite updates site metadata (domains).
// PUT /api/console/sites/:name
func (h *ConsoleHandler) HandleUpdateSite(c *gin.Context) {
	siteName := c.Param("name")
	if siteName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "site name required"}})
		return
	}

	var req struct {
		Domains []string `json:"domains"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request"}})
		return
	}

	site := h.SiteRouter.SiteByName(siteName)
	if site == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Site not found: " + siteName}})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "site name required"}})
		return
	}

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request"}})
		return
	}
	if req.Confirm != siteName {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Type the site hostname to confirm deletion."}})
		return
	}

	loaded := h.SiteRouter.SiteByName(siteName)
	if loaded == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Site not found: " + siteName}})
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to delete site: " + err.Error()}})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "site name required"}})
		return
	}

	var req struct {
		Email       string `json:"email"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "Invalid request"}})
		return
	}
	if req.Email == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: map[string]string{"message": "email and new_password are required"}})
		return
	}

	loaded := h.SiteRouter.SiteByName(siteName)
	if loaded == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "Site not found: " + siteName}})
		return
	}

	// Hash the new password.
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("hashing password failed", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to hash password."}})
		return
	}

	// Update the user's password in the site's database.
	result, err := loaded.DB.Exec(
		"UPDATE _kora_user SET password_hash = ?, modified = CURRENT_TIMESTAMP WHERE email = ?",
		passwordHash, req.Email,
	)
	if err != nil {
		slog.Error("resetting site password failed", "site", siteName, "email", req.Email, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: map[string]string{"message": "Failed to update password: " + err.Error()}})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: map[string]string{"message": "No user found with email: " + req.Email}})
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
