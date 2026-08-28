package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asenawritescode/kora/analytics"
	"github.com/asenawritescode/kora/email"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// dbTimeout is the maximum time a database query can take before being cancelled.
// Prevents indefinite hangs when the database server is slow or overloaded.
const dbTimeout = 5 * time.Second
const dbCleanupTimeout = 10 * time.Second // cleanup queries get longer timeout
const magicLinkLifetime = 15 * time.Minute
const magicLinkCleanupGrace = 24 * time.Hour
const authRateLimitWindow = 10 * time.Minute
const authRateLimitMax = 8

// User represents an authenticated user.
type User struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Roles    []string `json:"roles"`
	Enabled  bool     `json:"enabled"`
}

// SessionLifetime is the duration for which sessions are valid. Set before creating SessionManagers.
var SessionLifetime = 24 * time.Hour

// sessionCacheTTL is how long session lookups are cached before re-validating.
const sessionCacheTTL = 30 * time.Second
const sessionCacheCleanupInterval = 5 * time.Minute

type sessionCacheEntry struct {
	user      *User
	site      string // site the session belongs to (prevents cross-site cache hits)
	cachedAt  time.Time
	createdAt time.Time
	expiresAt time.Time // session expiry from DB
}

// SessionManager manages user sessions with an in-memory TTL cache.
type SessionManager struct {
	DB      *sql.DB
	cacheMu sync.RWMutex
	cache   map[string]*sessionCacheEntry
}

// sessionManagerRegistry caches SessionManagers per *sql.DB to prevent
// goroutine leaks from per-request SessionManager creation. Each *sql.DB
// gets exactly one SessionManager with one sweep goroutine.
var (
	sessionManagerRegistry   = make(map[*sql.DB]*SessionManager)
	sessionManagerRegistryMu sync.Mutex
)

// NewSessionManager returns a shared SessionManager for the given *sql.DB.
// If db is nil (console-only mode with no sites), returns a SessionManager
// with no sweep goroutine and no shared caching — each call is independent.
// For non-nil db, the same SessionManager is returned for the same *sql.DB
// pointer, ensuring exactly one sweepCacheLoop goroutine per database.
func NewSessionManager(db *sql.DB) *SessionManager {
	if db == nil {
		return &SessionManager{
			DB:    nil,
			cache: make(map[string]*sessionCacheEntry),
		}
	}

	sessionManagerRegistryMu.Lock()
	defer sessionManagerRegistryMu.Unlock()

	if sm, ok := sessionManagerRegistry[db]; ok {
		return sm
	}

	sm := &SessionManager{
		DB:    db,
		cache: make(map[string]*sessionCacheEntry),
	}
	go sm.sweepCacheLoop()
	sessionManagerRegistry[db] = sm
	return sm
}

// CreateSession creates a new session for a user and returns the session ID.
func (sm *SessionManager) CreateSession(site string, user *User) (string, error) {
	if sm.DB == nil {
		return "", fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	sid := generateSessionID()
	expiresAt := time.Now().Add(SessionLifetime)

	// Marshal user data as JSON in Go (dialect-neutral, avoids MySQL-only JSON_OBJECT).
	userData := gin.H{
		"name":      user.Name,
		"email":     user.Email,
		"full_name": user.FullName,
		"roles":     user.Roles,
	}
	userJSON, err := json.Marshal(userData)
	if err != nil {
		return "", fmt.Errorf("marshaling user data: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	_, err = sm.DB.ExecContext(ctx,
		`INSERT INTO _kora_session (sid, site, user, data, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sid, site, user.Name, string(userJSON), expiresAt, time.Now(),
	)
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}

	return sid, nil
}

// GetSession validates a session ID for a given site and returns the associated user
// plus the session creation timestamp.
// Uses an in-memory TTL cache to avoid hitting the database on every request.
func (sm *SessionManager) GetSession(site, sid string) (*User, time.Time, error) {
	if sm.DB == nil {
		return nil, time.Time{}, fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	// Check cache first.
	sm.cacheMu.RLock()
	entry, ok := sm.cache[sid]
	sm.cacheMu.RUnlock()

	if ok && time.Now().Before(entry.cachedAt.Add(sessionCacheTTL)) && entry.site == site {
		if time.Now().After(entry.expiresAt) {
			sm.DeleteSession(sid)
			return nil, time.Time{}, fmt.Errorf("session expired: %w", ErrSessionExpired)
		}
		return entry.user, entry.createdAt, nil
	}

	// Cache miss or expired — query database.
	var userJSON string
	var expiresStr string // scanned as string for SQLite compatibility (TEXT column)
	var createdStr string

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	err := sm.DB.QueryRowContext(ctx,
		"SELECT data, expires_at, created_at FROM _kora_session WHERE site = ? AND sid = ?",
		site, sid,
	).Scan(&userJSON, &expiresStr, &createdStr)

	if err == sql.ErrNoRows {
		// Remove from cache if present.
		sm.cacheMu.Lock()
		delete(sm.cache, sid)
		sm.cacheMu.Unlock()
		return nil, time.Time{}, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("querying session: %w", err)
	}

	expiresAt, err := parseTime(expiresStr)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parsing session expiry: %w", err)
	}
	createdAt, err := parseTime(createdStr)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parsing session creation time: %w", err)
	}

	if time.Now().After(expiresAt) {
		sm.DeleteSession(sid)
		return nil, time.Time{}, fmt.Errorf("session expired: %w", ErrSessionExpired)
	}

	// Parse JSON. For simplicity in Phase 1, parse manually.
	user := &User{}
	if err := scanUserJSON(userJSON, user); err != nil {
		return nil, time.Time{}, fmt.Errorf("parsing session data: %w", err)
	}

	// Populate cache.
	sm.cacheMu.Lock()
	sm.cache[sid] = &sessionCacheEntry{
		user:      user,
		site:      site,
		cachedAt:  time.Now(),
		createdAt: createdAt,
		expiresAt: expiresAt,
	}
	sm.cacheMu.Unlock()

	return user, createdAt, nil
}

func scanUserJSON(jsonStr string, user *User) error {
	// Simple JSON parsing — extract name, email, full_name, roles.
	extract := func(key string) string {
		start := 0
		for {
			idx := indexAfter(jsonStr, `"`+key+`"`, start)
			if idx < 0 {
				return ""
			}
			// Skip whitespace and colon.
			rest := jsonStr[idx:]
			colonIdx := 0
			for colonIdx < len(rest) && (rest[colonIdx] == ' ' || rest[colonIdx] == ':') {
				colonIdx++
			}
			if colonIdx >= len(rest) {
				return ""
			}
			rest = rest[colonIdx:]
			if len(rest) > 0 && rest[0] == '[' {
				// Array value.
				endIdx := 1
				for endIdx < len(rest) && rest[endIdx] != ']' {
					endIdx++
				}
				return rest[:endIdx+1]
			}
			if len(rest) > 0 && rest[0] == '"' {
				// String value.
				endIdx := 1
				for endIdx < len(rest) && rest[endIdx] != '"' {
					endIdx++
				}
				return rest[1:endIdx]
			}
			start = idx + len(key) + 2
		}
	}

	user.Name = extract("name")
	user.Email = extract("email")
	user.FullName = extract("full_name")
	rolesStr := extract("roles")
	if rolesStr != "" {
		// Parse array: ["Role1", "Role2"]
		rolesStr = trim(rolesStr, "[]")
		if rolesStr != "" {
			parts := splitQuoted(rolesStr)
			for _, p := range parts {
				user.Roles = append(user.Roles, p)
			}
		}
	}

	return nil
}

func indexAfter(s, substr string, start int) int {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i + len(substr)
		}
	}
	return -1
}

func trim(s, cutset string) string {
	for len(s) > 0 && contains(cutset, s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && contains(cutset, s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

func contains(set string, c byte) bool {
	for i := 0; i < len(set); i++ {
		if set[i] == c {
			return true
		}
	}
	return false
}

func splitQuoted(s string) []string {
	var result []string
	var current string
	inQuote := false
	for _, c := range s {
		if c == '"' {
			inQuote = !inQuote
			if !inQuote && current != "" {
				result = append(result, current)
				current = ""
			}
		} else if inQuote {
			current += string(c)
		}
	}
	return result
}

// DeleteSession removes a session.
func (sm *SessionManager) DeleteSession(sid string) {
	if sm.DB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	_, err := sm.DB.ExecContext(ctx, "DELETE FROM _kora_session WHERE sid = ?", sid)
	if err != nil {
		slog.Warn("failed to delete session", "sid", sid, "error", err)
	}
	// Invalidate cache entry.
	sm.cacheMu.Lock()
	delete(sm.cache, sid)
	sm.cacheMu.Unlock()
}

// InvalidateSession removes a session from the cache without deleting it from the database.
// Use this when user state changes (role change, password change, disable) to force re-validation.
func (sm *SessionManager) InvalidateSession(sid string) {
	sm.cacheMu.Lock()
	delete(sm.cache, sid)
	sm.cacheMu.Unlock()
}

// sweepCacheLoop periodically removes expired cache entries and cleans up expired DB sessions.
func (sm *SessionManager) sweepCacheLoop() {
	ticker := time.NewTicker(sessionCacheCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		// Clean in-memory cache.
		sm.cacheMu.Lock()
		now := time.Now()
		for sid, entry := range sm.cache {
			if now.After(entry.expiresAt) || now.After(entry.cachedAt.Add(sessionCacheTTL*2)) {
				delete(sm.cache, sid)
			}
		}
		sm.cacheMu.Unlock()

		// Clean expired DB sessions.
		sm.cleanupExpired()
		sm.cleanupMagicLinks()
		cleanupAuthRateLimits()
	}
}

func (sm *SessionManager) cleanupExpired() {
	if sm.DB == nil {
		return // console-only mode — no site database
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbCleanupTimeout)
	defer cancel()
	_, err := sm.DB.ExecContext(ctx, "DELETE FROM _kora_session WHERE expires_at < ?", time.Now())
	if err != nil {
		slog.Warn("failed to cleanup expired sessions", "error", err)
	}
}

func (sm *SessionManager) cleanupMagicLinks() {
	if sm.DB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbCleanupTimeout)
	defer cancel()
	cutoff := time.Now().Add(-magicLinkCleanupGrace)
	_, err := sm.DB.ExecContext(ctx,
		`DELETE FROM _kora_magic_link
		 WHERE expires_at < ? OR revoked_at < ? OR (used_at IS NOT NULL AND used_at < ?)`,
		time.Now(), cutoff, cutoff,
	)
	if err != nil {
		slog.Warn("failed to cleanup magic links", "error", err)
	}
}

// AuthenticateUser verifies a username/email and password against the database.
func (sm *SessionManager) AuthenticateUser(site, email, password string) (*User, error) {
	if sm.DB == nil {
		return nil, fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	var name, emailAddr, passwordHash, fullName, rolesStr string
	var enabled bool

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	err := sm.DB.QueryRowContext(ctx,
		"SELECT name, email, password_hash, full_name, enabled, COALESCE(roles, '') FROM _kora_user WHERE site = ? AND email = ?",
		site, email,
	).Scan(&name, &emailAddr, &passwordHash, &fullName, &enabled, &rolesStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w", ErrInvalidCredentials)
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}

	if !enabled {
		// Return generic "invalid credentials" to prevent user enumeration.
		// The real reason is logged for audit purposes.
		slog.Warn("login attempt for disabled account", "email", email)
		return nil, fmt.Errorf("%w", ErrInvalidCredentials)
	}

	// Verify password.
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("%w", ErrInvalidCredentials)
	}

	// Parse roles.
	var roles []string
	if rolesStr != "" {
		// Simple comma-separated or newline-separated.
		parts := splitRolesStr(rolesStr)
		roles = parts
	}

	return &User{
		Name:     name,
		Email:    emailAddr,
		FullName: fullName,
		Roles:    roles,
		Enabled:  enabled,
	}, nil
}

func (sm *SessionManager) FindUserByEmail(site, emailAddr string) (*User, error) {
	if sm.DB == nil {
		return nil, fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	var name, storedEmail, fullName, rolesStr string
	var enabled bool

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	err := sm.DB.QueryRowContext(ctx,
		"SELECT name, email, full_name, enabled, COALESCE(roles, '') FROM _kora_user WHERE site = ? AND email = ?",
		site, emailAddr,
	).Scan(&name, &storedEmail, &fullName, &enabled, &rolesStr)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	if !enabled {
		slog.Warn("magic-link request for disabled account", "email", emailAddr)
		return nil, ErrDisabledAccount
	}

	var roles []string
	if rolesStr != "" {
		roles = splitRolesStr(rolesStr)
	}

	return &User{
		Name:     name,
		Email:    storedEmail,
		FullName: fullName,
		Roles:    roles,
		Enabled:  enabled,
	}, nil
}

func (sm *SessionManager) IsEmailVerified(site, emailAddr string) (bool, error) {
	if sm.DB == nil {
		return false, fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}

	var verified sql.NullTime
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	err := sm.DB.QueryRowContext(ctx,
		"SELECT email_verified_at FROM _kora_user WHERE site = ? AND email = ?",
		site, emailAddr,
	).Scan(&verified)
	if err == sql.ErrNoRows {
		return false, ErrInvalidCredentials
	}
	if err != nil {
		return false, fmt.Errorf("querying user verification state: %w", err)
	}
	return verified.Valid, nil
}

func (sm *SessionManager) MarkEmailVerified(site, emailAddr string) error {
	if sm.DB == nil {
		return fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	_, err := sm.DB.ExecContext(ctx,
		`UPDATE _kora_user
		 SET email_verified_at = COALESCE(email_verified_at, ?), modified = CURRENT_TIMESTAMP
		 WHERE site = ? AND email = ?`,
		time.Now(), site, emailAddr,
	)
	if err != nil {
		return fmt.Errorf("marking email verified: %w", err)
	}
	return nil
}

type MagicLinkRecord struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func (sm *SessionManager) ListMagicLinks(site, emailAddr string) ([]MagicLinkRecord, error) {
	if sm.DB == nil {
		return nil, fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	rows, err := sm.DB.QueryContext(ctx,
		`SELECT id, email, created_at, expires_at, used_at, revoked_at
		 FROM _kora_magic_link
		 WHERE site = ? AND email = ? AND (revoked_at IS NULL OR revoked_at >= ?) AND expires_at >= ?
		 ORDER BY created_at DESC`,
		site, emailAddr, time.Now().Add(-magicLinkCleanupGrace), time.Now().Add(-magicLinkCleanupGrace),
	)
	if err != nil {
		return nil, fmt.Errorf("querying magic links: %w", err)
	}
	defer rows.Close()

	var links []MagicLinkRecord
	for rows.Next() {
		var rec MagicLinkRecord
		var usedAt, revokedAt sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.Email, &rec.CreatedAt, &rec.ExpiresAt, &usedAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scanning magic link: %w", err)
		}
		if usedAt.Valid {
			v := usedAt.Time
			rec.UsedAt = &v
		}
		if revokedAt.Valid {
			v := revokedAt.Time
			rec.RevokedAt = &v
		}
		links = append(links, rec)
	}
	return links, nil
}

func (sm *SessionManager) RevokeMagicLink(site, emailAddr, id string) error {
	if sm.DB == nil {
		return fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	_, err := sm.DB.ExecContext(ctx,
		`UPDATE _kora_magic_link
		 SET revoked_at = COALESCE(revoked_at, ?)
		 WHERE site = ? AND email = ? AND id = ?`,
		time.Now(), site, emailAddr, id,
	)
	if err != nil {
		return fmt.Errorf("revoking magic link: %w", err)
	}
	return nil
}

func (sm *SessionManager) RevokeAllMagicLinks(site, emailAddr string) error {
	if sm.DB == nil {
		return fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	_, err := sm.DB.ExecContext(ctx,
		`UPDATE _kora_magic_link
		 SET revoked_at = COALESCE(revoked_at, ?)
		 WHERE site = ? AND email = ? AND revoked_at IS NULL AND used_at IS NULL`,
		time.Now(), site, emailAddr,
	)
	if err != nil {
		return fmt.Errorf("revoking magic links: %w", err)
	}
	return nil
}

func splitRolesStr(s string) []string {
	// Try newline first, then comma.
	if containsStr(s, "\n") {
		result := splitStr(s, "\n")
		for i, r := range result {
			result[i] = trimWhitespace(r)
		}
		return result
	}
	result := splitStr(s, ",")
	for i, r := range result {
		result[i] = trimWhitespace(r)
	}
	return result
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && indexAfterStr(s, substr) >= 0
}

func indexAfterStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func splitStr(s, sep string) []string {
	var result []string
	for {
		idx := indexAfterStr(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func trimWhitespace(s string) string {
	s = trim(s, " \t\r\n")
	return s
}

// HashPassword creates a bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateMagicLinkToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var generateMagicLinkTokenFn = generateMagicLinkToken

func hashMagicLinkToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// AuthMiddleware returns a Gin middleware that validates session cookies.
func AuthMiddleware(sm *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for login endpoint and health check.
		path := c.Request.URL.Path
		if path == "/api/auth/login" ||
			path == "/api/auth/providers" ||
			path == "/api/auth/magic-link/request" ||
			path == "/api/auth/magic-link/verify" ||
			path == "/api/ping" ||
			path == "/workspace/login" ||
			path == "/workspace/auth/login" ||
			path == "/console/login" {
			c.Next()
			return
		}

		if !validateSession(c, sm) {
			return
		}
		c.Next()
	}
}

// validateSession validates the session cookie/header and sets user context.
// Returns false and writes 401 if auth fails; true if auth succeeded or was skipped.
// Does NOT call c.Next() — callers must do that themselves.
func validateSession(c *gin.Context, sm *SessionManager) bool {
	// Get session cookie.
	sid, err := c.Cookie("kora_sid")
	if err != nil {
		// Try Authorization header.
		authHeader := c.GetHeader("Authorization")
		if stringsHasPrefix(authHeader, "Bearer ") {
			sid = authHeader[7:]
		}
	}

	if sid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		c.Abort()
		return false
	}

	// Use site-specific DB for session validation if available.
	sessionSM := sm
	if siteDB, exists := c.Get("site_db"); exists {
		if sdb, ok := siteDB.(*sql.DB); ok && sdb != sm.DB {
			sessionSM = NewSessionManager(sdb)
		}
	}

	site := c.GetString("site_name")
	user, createdAt, err := sessionSM.GetSession(site, sid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
		c.Abort()
		return false
	}

	c.Set("user", user.Name)
	c.Set("user_obj", user)
	c.Set("session_sid", sid)
	c.Set("session_created_at", createdAt)

	// Set role info for permission checks.
	if len(user.Roles) > 0 {
		c.Set("user_role", user.Roles[0])
		c.Set("user_roles", user.Roles)
	} else {
		c.Set("user_role", "")
		c.Set("user_roles", []string{})
	}
	return true
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func requireAuthenticatedAuthUser(c *gin.Context, sm *SessionManager) (*SessionManager, *User, bool) {
	targetSM := sm
	if siteDB, exists := c.Get("site_db"); exists {
		if sdb, ok := siteDB.(*sql.DB); ok && sdb != nil && sdb != sm.DB {
			targetSM = NewSessionManager(sdb)
		}
	}
	if !validateSession(c, targetSM) {
		return nil, nil, false
	}
	userVal, exists := c.Get("user_obj")
	user, ok := userVal.(*User)
	if !exists || !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		c.Abort()
		return nil, nil, false
	}
	return targetSM, user, true
}

// RegisterAuthRoutes registers authentication endpoints.
func RegisterAuthRoutes(router *gin.Engine, sm *SessionManager, db *sql.DB, mailer *email.Sender) {
	registerAuthRoutes(router, sm, db, mailer, NewProviderRegistry())
}

func registerAuthRoutes(router *gin.Engine, sm *SessionManager, db *sql.DB, mailer *email.Sender, registry *ProviderRegistry) {
	if registry == nil {
		registry = NewProviderRegistry()
	}
	auth := router.Group("/api/auth")
	{
		auth.POST("/login", func(c *gin.Context) {
			var req struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
				return
			}
			emailAddr := strings.TrimSpace(req.Email)
			emailKey := strings.ToLower(emailAddr)
			if emailAddr == "" || strings.TrimSpace(req.Password) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
				return
			}
			site := c.GetString("site_name")
			if !enforceAuthRateLimit(c, fmt.Sprintf("login:%s:%s:%s", site, c.ClientIP(), emailKey), authRateLimitMax, authRateLimitWindow) {
				return
			}

			// Use site-specific DB if available (multi-site path-based or Host-based).
			db := db
			if siteDB, exists := c.Get("site_db"); exists {
				if sdb, ok := siteDB.(*sql.DB); ok {
					db = sdb
				}
			}
			sm := NewSessionManager(db)

			user, err := sm.AuthenticateUser(site, emailAddr, req.Password)
			if err != nil {
				// Only return known user-facing messages; log internal errors.
				msg := err.Error()
				if msg != "invalid credentials" {
					slog.Error("login authentication error", "error", err)
					msg = "invalid credentials"
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
				return
			}

			verified, err := sm.IsEmailVerified(site, user.Email)
			if err != nil {
				slog.Error("checking email verification state failed", "error", err, "email", user.Email)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify account status"})
				return
			}
			if !verified {
				if _, err := issueOneTimeLink(c, db, site, user, mailer); err != nil {
					slog.Error("issuing verification link failed", "error", err, "email", user.Email)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send verification link"})
					return
				}
				c.JSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"type":    "email_verification_required",
						"message": "We sent a verification link to your email address. Open it to finish signing in.",
					},
				})
				return
			}

			sid, err := sm.CreateSession(site, user)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
				return
			}

			// Set cookie with Secure auto-detected from TLS and SameSite=Lax.
			SetSecureCookie(c, "kora_sid", sid, int(SessionLifetime.Seconds()), "/", true)

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"name":      user.Name,
					"email":     user.Email,
					"full_name": user.FullName,
					"roles":     user.Roles,
				},
				"sid": sid,
			})

			if cfg := analytics.LoadCloudRelayConfig(); cfg != nil {
				analytics.SendCloudProductEvent(nil, *cfg, analytics.CloudProductEventDTO{
					SiteID:    site,
					AccountID: cfg.AccountID,
					Kind:      "first_login",
					Properties: analytics.FirstLoginPropertiesDTO{
						User:  user.Email,
						Site:  site,
						Login: "password",
					},
				}, "first_login:"+site+":"+user.Email)
			}
		})

		auth.POST("/logout", func(c *gin.Context) {
			sid, _ := c.Cookie("kora_sid")
			if sid != "" {
				// Use site DB if available.
				logoutSM := sm
				if siteDB, exists := c.Get("site_db"); exists {
					if sdb, ok := siteDB.(*sql.DB); ok {
						logoutSM = NewSessionManager(sdb)
					}
				}
				logoutSM.DeleteSession(sid)
			}
			SetSecureCookie(c, "kora_sid", "", -1, "/", true)
			c.JSON(http.StatusOK, gin.H{"message": "logged out"})
		})

		auth.GET("/providers", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"providers": registry.List(),
				},
			})
		})

		auth.POST("/magic-link/request", func(c *gin.Context) {
			var req struct {
				Email string `json:"email"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Email) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
				return
			}
			emailAddr := strings.TrimSpace(req.Email)
			emailKey := strings.ToLower(emailAddr)
			site := c.GetString("site_name")
			if !enforceAuthRateLimit(c, fmt.Sprintf("magic-request:%s:%s:%s", site, c.ClientIP(), emailKey), authRateLimitMax, authRateLimitWindow) {
				return
			}

			targetDB := db
			if siteDB, exists := c.Get("site_db"); exists {
				if sdb, ok := siteDB.(*sql.DB); ok {
					targetDB = sdb
				}
			}
			targetSM := NewSessionManager(targetDB)

			user, err := targetSM.FindUserByEmail(site, emailAddr)
			if err == nil && user != nil {
				if _, err := issueOneTimeLink(c, targetDB, site, user, mailer); err != nil {
					slog.Error("creating magic link token failed", "email", user.Email, "error", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create magic link"})
					return
				}
			}

			c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a sign-in link has been sent."})
		})

		auth.POST("/magic-link/verify", func(c *gin.Context) {
			var req struct {
				Token string `json:"token"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
				return
			}
			if !enforceAuthRateLimit(c, fmt.Sprintf("magic-verify:%s:%s", c.GetString("site_name"), c.ClientIP()), authRateLimitMax, authRateLimitWindow) {
				return
			}

			site := c.GetString("site_name")
			targetDB := db
			if siteDB, exists := c.Get("site_db"); exists {
				if sdb, ok := siteDB.(*sql.DB); ok {
					targetDB = sdb
				}
			}
			targetSM := NewSessionManager(targetDB)

			tokenHash := hashMagicLinkToken(req.Token)
			var id, emailAddr string
			var usedFlag int
			var revokedFlag int
			var expiresAtRaw any
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			err := targetDB.QueryRowContext(ctx,
				`SELECT id, email, CASE WHEN used_at IS NULL THEN 0 ELSE 1 END, CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, expires_at FROM _kora_magic_link
				 WHERE site = ? AND token_hash = ?`,
				site, tokenHash,
			).Scan(&id, &emailAddr, &usedFlag, &revokedFlag, &expiresAtRaw)
			cancel()
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify magic link"})
				return
			}

			if usedFlag != 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}
			if revokedFlag != 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}
			expiresAt, err := parseDBTimeValue(expiresAtRaw)
			if err != nil || time.Now().After(expiresAt) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}

			ctx, cancel = context.WithTimeout(context.Background(), dbTimeout)
			result, err := targetDB.ExecContext(ctx,
				`UPDATE _kora_magic_link SET used_at = ? WHERE id = ? AND used_at IS NULL`,
				time.Now(), id,
			)
			cancel()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to consume magic link"})
				return
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to consume magic link"})
				return
			}
			if rowsAffected == 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}

			user, err := targetSM.FindUserByEmail(site, emailAddr)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
				return
			}
			if err := targetSM.MarkEmailVerified(site, emailAddr); err != nil {
				slog.Warn("failed to mark email verified", "email", emailAddr, "error", err)
			}

			sid, err := targetSM.CreateSession(site, user)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
				return
			}

			SetSecureCookie(c, "kora_sid", sid, int(SessionLifetime.Seconds()), "/", true)
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"name":      user.Name,
					"email":     user.Email,
					"full_name": user.FullName,
					"roles":     user.Roles,
				},
				"sid": sid,
			})
		})

		auth.GET("/magic-links", func(c *gin.Context) {
			targetSM, user, ok := requireAuthenticatedAuthUser(c, sm)
			if !ok {
				return
			}
			site := c.GetString("site_name")
			links, err := targetSM.ListMagicLinks(site, user.Email)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list magic links"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"links": links}})
		})

		auth.POST("/magic-links/:id/revoke", func(c *gin.Context) {
			targetSM, user, ok := requireAuthenticatedAuthUser(c, sm)
			if !ok {
				return
			}
			site := c.GetString("site_name")
			if err := targetSM.RevokeMagicLink(site, user.Email, c.Param("id")); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke magic link"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Magic link revoked"}})
		})

		auth.POST("/magic-links/revoke-all", func(c *gin.Context) {
			targetSM, user, ok := requireAuthenticatedAuthUser(c, sm)
			if !ok {
				return
			}
			site := c.GetString("site_name")
			if err := targetSM.RevokeAllMagicLinks(site, user.Email); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke magic links"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Magic links revoked"}})
		})

		auth.GET("/me", func(c *gin.Context) {
			sid, _ := c.Cookie("kora_sid")
			if sid == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
				return
			}
			// Use site DB if available.
			meSM := sm
			if siteDB, exists := c.Get("site_db"); exists {
				if sdb, ok := siteDB.(*sql.DB); ok {
					meSM = NewSessionManager(sdb)
				}
			}
			site := c.GetString("site_name")
			user, _, err := meSM.GetSession(site, sid)
			if err != nil {
				slog.Warn("session validation failed for /me", "error", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"name":      user.Name,
					"email":     user.Email,
					"full_name": user.FullName,
					"roles":     user.Roles,
				},
			})
		})
	}
}

// parseTime parses a datetime string from the database (MySQL DATETIME or SQLite TEXT).
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,                      // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02 15:04:05.999999999-07:00", // SQLite with nanoseconds + timezone
		"2006-01-02 15:04:05.999999-07:00",    // SQLite with microseconds + timezone
		"2006-01-02 15:04:05-07:00",           // SQLite with timezone
		"2006-01-02 15:04:05.999999",          // MySQL with microseconds
		"2006-01-02 15:04:05",                 // MySQL without fractional seconds
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

func parseDBTimeValue(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		return parseTime(t)
	case []byte:
		return parseTime(string(t))
	default:
		return time.Time{}, fmt.Errorf("unrecognized time value type %T", v)
	}
}

func buildMagicLinkURL(c *gin.Context, token string) string {
	scheme := "http"
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		host = c.Request.URL.Host
	}
	return fmt.Sprintf("%s://%s%s?magic_token=%s", scheme, host, magicLinkLoginPath(c), token)
}

func magicLinkLoginPath(c *gin.Context) string {
	site := strings.TrimSpace(c.GetString("site_name"))
	if site == "" {
		return "/workspace/auth/login"
	}
	if cookieSite, err := c.Cookie("kora_site"); err == nil && strings.TrimSpace(cookieSite) == site {
		return "/s/" + site + "/workspace/auth/login"
	}
	return "/workspace/auth/login"
}

func enforceAuthRateLimit(c *gin.Context, key string, limit int, window time.Duration) bool {
	ok, retryAfter := allowAuthRequest(key, limit, window)
	if ok {
		return true
	}
	if retryAfter > 0 {
		c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
	}
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"type":    "rate_limited",
			"message": "Too many authentication attempts. Please try again later.",
		},
	})
	return false
}

func issueOneTimeLink(c *gin.Context, targetDB *sql.DB, site string, user *User, mailer *email.Sender) (string, error) {
	if targetDB == nil {
		return "", fmt.Errorf("no database connection available: %w", ErrNoDBConnection)
	}

	token := generateMagicLinkTokenFn()
	tokenHash := hashMagicLinkToken(token)
	expiresAt := time.Now().Add(magicLinkLifetime)
	linkID := "ml-" + generateSessionID()
	linkURL := buildMagicLinkURL(c, token)

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_, err := targetDB.ExecContext(ctx,
		`INSERT INTO _kora_magic_link (id, site, email, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		linkID, site, user.Email, tokenHash, expiresAt, time.Now(),
	)
	cancel()
	if err != nil {
		return "", fmt.Errorf("creating magic link token failed: %w", err)
	}

	subject, textBody, htmlBody := email.MagicLinkTemplate("Kora", linkURL, int(magicLinkLifetime.Minutes()))
	if mailer != nil {
		if err := mailer.Send(&email.Message{
			To:       []string{user.Email},
			Subject:  subject,
			TextBody: textBody,
			HTMLBody: htmlBody,
		}); err != nil {
			slog.Warn("sending magic link email failed", "email", user.Email, "error", err)
		}
	} else {
		slog.Info("magic link generated", "email", user.Email, "link", linkURL)
	}
	return token, nil
}
