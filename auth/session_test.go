package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestBuildMagicLinkURL_PathBasedSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "https://app.kora.test/api/auth/magic-link/request", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "app.kora.test")
	req.AddCookie(&http.Cookie{Name: "kora_site", Value: "live-demo"})
	c.Request = req
	c.Set("site_name", "live-demo")

	got := buildMagicLinkURL(c, "abc123")
	want := "https://app.kora.test/s/live-demo/workspace/auth/login?magic_token=abc123"
	if got != want {
		t.Fatalf("buildMagicLinkURL() = %q, want %q", got, want)
	}
}

func TestBuildMagicLinkURL_HostBasedSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "https://live-demo.kora.test/api/auth/magic-link/request", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "live-demo.kora.test")
	c.Request = req
	c.Set("site_name", "live-demo")

	got := buildMagicLinkURL(c, "abc123")
	want := "https://live-demo.kora.test/workspace/auth/login?magic_token=abc123"
	if got != want {
		t.Fatalf("buildMagicLinkURL() = %q, want %q", got, want)
	}
}

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"simple", "password123", false},
		{"complex", "P@ssw0rd!-#&$%", false},
		{"empty", "", false},
		{"long", "a-very-long-password-that-should-still-work-fine-with-bcrypt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("HashPassword() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && hash == "" {
				t.Fatal("HashPassword returned empty hash")
			}
			if tt.password != "" {
				// Verify bcrypt round-trip.
				err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(tt.password))
				if err != nil {
					t.Errorf("bcrypt compare failed: %v", err)
				}
			}
		})
	}
}

func TestAuthenticateUser_ValidCredentials(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	hash, _ := HashPassword("validpass")
	rows := sqlmock.NewRows([]string{"name", "email", "password_hash", "full_name", "enabled", "roles"}).
		AddRow("john", "john@test.com", hash, "John Doe", true, "Admin,Editor")

	mock.ExpectQuery("SELECT name, email, password_hash, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "john@test.com").
		WillReturnRows(rows)

	user, err := sm.AuthenticateUser("test.local", "john@test.com", "validpass")
	if err != nil {
		t.Fatalf("AuthenticateUser error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("user.Name = %q, want %q", user.Name, "john")
	}
	if user.Email != "john@test.com" {
		t.Errorf("user.Email = %q, want %q", user.Email, "john@test.com")
	}
	if user.FullName != "John Doe" {
		t.Errorf("user.FullName = %q, want %q", user.FullName, "John Doe")
	}
	if !user.Enabled {
		t.Error("user.Enabled should be true")
	}
	if len(user.Roles) == 0 || user.Roles[0] != "Admin" {
		t.Errorf("user.Roles = %v, want [Admin Editor]", user.Roles)
	}
}

func TestAuthenticateUser_InvalidPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	hash, _ := HashPassword("correctpass")
	rows := sqlmock.NewRows([]string{"name", "email", "password_hash", "full_name", "enabled", "roles"}).
		AddRow("john", "john@test.com", hash, "John Doe", true, "Admin")

	mock.ExpectQuery("SELECT name, email, password_hash, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "john@test.com").
		WillReturnRows(rows)

	_, err = sm.AuthenticateUser("test.local", "john@test.com", "wrongpass")
	if err == nil {
		t.Fatal("AuthenticateUser should error for wrong password")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid credentials")
	}
}

func TestAuthenticateUser_UnknownEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	mock.ExpectQuery("SELECT name, email, password_hash, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "unknown@test.com").
		WillReturnError(sql.ErrNoRows)

	_, err = sm.AuthenticateUser("test.local", "unknown@test.com", "anypass")
	if err == nil {
		t.Fatal("AuthenticateUser should error for unknown email")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid credentials")
	}
}

func TestAuthenticateUser_DisabledAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	hash, _ := HashPassword("somepass")
	rows := sqlmock.NewRows([]string{"name", "email", "password_hash", "full_name", "enabled", "roles"}).
		AddRow("john", "john@test.com", hash, "John Doe", false, "Admin")

	mock.ExpectQuery("SELECT name, email, password_hash, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "john@test.com").
		WillReturnRows(rows)

	_, err = sm.AuthenticateUser("test.local", "john@test.com", "somepass")
	if err == nil {
		t.Fatal("AuthenticateUser should error for disabled account")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid credentials")
	}
}

func TestAuthenticateUser_NoDatabase(t *testing.T) {
	sm := NewSessionManager(nil)
	_, err := sm.AuthenticateUser("test.local", "user@test.com", "pass")
	if err == nil {
		t.Fatal("AuthenticateUser should error when DB is nil")
	}
}

func TestCreateSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)
	SessionLifetime = 24 * time.Hour

	user := &User{
		Name:     "john",
		Email:    "john@test.com",
		FullName: "John Doe",
		Roles:    []string{"Admin"},
		Enabled:  true,
	}

	mock.ExpectExec("INSERT INTO _kora_session").
		WithArgs(sqlmock.AnyArg(), "test.local", "john", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	sid, err := sm.CreateSession("test.local", user)
	if err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}
	if sid == "" {
		t.Fatal("CreateSession returned empty sid")
	}
	if len(sid) != 64 {
		t.Errorf("sid length = %d, want 64", len(sid))
	}
}

func TestCreateSession_NoDatabase(t *testing.T) {
	sm := NewSessionManager(nil)
	_, err := sm.CreateSession("test.local", &User{Name: "test"})
	if err == nil {
		t.Fatal("CreateSession should error when DB is nil")
	}
}

func TestDeleteSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	mock.ExpectExec("DELETE FROM _kora_session WHERE sid = ?").
		WithArgs("test-sid-123").
		WillReturnResult(sqlmock.NewResult(0, 1))

	sm.DeleteSession("test-sid-123")
	// No error — just ensure no panic and mock expectations are met.
}

func TestDeleteSession_NoDatabase(t *testing.T) {
	sm := NewSessionManager(nil)
	// Should not panic with nil DB.
	sm.DeleteSession("some-sid")
}

func TestGetSession_CacheHit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)

	// Pre-populate cache.
	user := &User{Name: "cached-user", Email: "cached@test.com", Roles: []string{"Admin"}, Enabled: true}
	sm.cacheMu.Lock()
	sm.cache["cached-sid"] = &sessionCacheEntry{
		user:      user,
		site:      "test-site",
		cachedAt:  time.Now(),
		expiresAt: time.Now().Add(1 * time.Hour),
	}
	sm.cacheMu.Unlock()

	// GetSession from cache should NOT hit the database.
	got, err := sm.GetSession("test-site", "cached-sid")
	if err != nil {
		t.Fatalf("GetSession error = %v", err)
	}
	if got.Name != "cached-user" {
		t.Errorf("user.Name = %q, want %q", got.Name, "cached-user")
	}

	// Ensure we didn't make any DB calls.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CSRF Middleware Tests
// ---------------------------------------------------------------------------

func TestCSRFMiddleware_SkipBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should skip CSRF for Bearer auth)", w.Code, http.StatusOK)
	}
}

func TestCSRFMiddleware_BlockNoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (should block POST without CSRF token)", w.Code, http.StatusForbidden)
	}
}

func TestCSRFMiddleware_SkipSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.HEAD("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/test", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", method, w.Code, http.StatusOK)
		}
	}
}

func TestCSRFMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Kora-CSRF-Token", "valid-csrf-token")
	req.Header.Set("Cookie", "kora_csrf=valid-csrf-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCSRFMiddleware_MismatchedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Kora-CSRF-Token", "token-from-header")
	req.Header.Set("Cookie", "kora_csrf=token-from-cookie")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// Helper to create a test HTTP request with body.
func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"mysql format", "2024-01-15 10:30:00", false},
		{"mysql with microseconds", "2024-01-15 10:30:00.123456", false},
		{"rfc3339", "2024-01-15T10:30:00Z", false},
		{"sqlite with tz", "2024-01-15 10:30:00+00:00", false},
		{"empty string", "", true},
		{"invalid", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && parsed.IsZero() {
				t.Error("parsed time should not be zero")
			}
		})
	}
}

func TestSplitRolesStr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"comma separated", "Admin,Editor,Viewer", []string{"Admin", "Editor", "Viewer"}},
		{"newline separated", "Admin\nEditor\nViewer", []string{"Admin", "Editor", "Viewer"}},
		{"single role", "Admin", []string{"Admin"}},
		{"empty string", "", []string{""}},
		{"with whitespace", " Admin , Editor ", []string{"Admin", "Editor"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRolesStr(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Ensure we can use errors in tests.
var _ = errors.New
var _ = strings.TrimSpace

func TestMagicLinks_ListRequiresSessionValidationWithinAuthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("site_name", "test.local")
		c.Set("site_db", db)
		c.Next()
	})
	RegisterAuthRoutes(router, sm, db, nil)

	userJSON, err := json.Marshal(map[string]any{
		"name":      "john",
		"email":     "john@test.com",
		"full_name": "John Doe",
		"roles":     []string{"Administrator"},
	})
	if err != nil {
		t.Fatalf("marshal user json: %v", err)
	}
	expiresAt := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
	mock.ExpectQuery("SELECT data, expires_at FROM _kora_session WHERE site = \\? AND sid = \\?").
		WithArgs("test.local", "valid-session").
		WillReturnRows(sqlmock.NewRows([]string{"data", "expires_at"}).AddRow(string(userJSON), expiresAt))
	mock.ExpectQuery("SELECT id, email, created_at, expires_at, used_at, revoked_at FROM _kora_magic_link").
		WithArgs("test.local", "john@test.com", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "created_at", "expires_at", "used_at", "revoked_at"}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-links", nil)
	req.AddCookie(&http.Cookie{Name: "kora_sid", Value: "valid-session"})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMagicLinkVerify_RejectsReusedLinkWhenConsumeAffectsZeroRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sm := NewSessionManager(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("site_name", "test.local")
		c.Set("site_db", db)
		c.Next()
	})
	RegisterAuthRoutes(router, sm, db, nil)

	token := "magic-token"
	tokenHash := hashMagicLinkToken(token)
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	mock.ExpectQuery("SELECT id, email, CASE WHEN used_at IS NULL THEN 0 ELSE 1 END, CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, expires_at FROM _kora_magic_link").
		WithArgs("test.local", tokenHash).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "used_flag", "revoked_flag", "expires_at"}).
			AddRow("ml-1", "john@test.com", 0, 0, future))
	mock.ExpectExec("UPDATE _kora_magic_link SET used_at = \\? WHERE id = \\? AND used_at IS NULL").
		WithArgs(sqlmock.AnyArg(), "ml-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"token":"magic-token"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "failed to create session") {
		t.Fatalf("unexpected session creation path reached: %s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMagicLinkLifecycle_EndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	oldTokenFn := generateMagicLinkTokenFn
	generateMagicLinkTokenFn = func() string { return "fixed-magic-token" }
	defer func() { generateMagicLinkTokenFn = oldTokenFn }()

	sm := NewSessionManager(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("site_name", "test.local")
		c.Set("site_db", db)
		c.Next()
	})
	RegisterAuthRoutes(router, sm, db, nil)

	magicLinkRows := sqlmock.NewRows([]string{"id", "email", "created_at", "expires_at", "used_at", "revoked_at"}).
		AddRow("ml-1", "john@test.com", time.Now().Add(-time.Minute), time.Now().Add(time.Hour), nil, nil)
	sessionRows := sqlmock.NewRows([]string{"data", "expires_at"}).
		AddRow(`{"name":"john","email":"john@test.com","full_name":"John Doe","roles":["Administrator"]}`, time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"))

	mock.ExpectQuery("SELECT name, email, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "john@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"name", "email", "full_name", "enabled", "roles"}).
			AddRow("john", "john@test.com", "John Doe", true, "Administrator"))
	mock.ExpectExec("INSERT INTO _kora_magic_link \\(id, site, email, token_hash, expires_at, created_at\\)").
		WithArgs(sqlmock.AnyArg(), "test.local", "john@test.com", hashMagicLinkToken("fixed-magic-token"), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link/request", strings.NewReader(`{"email":"john@test.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("request status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	mock.ExpectQuery("SELECT id, email, CASE WHEN used_at IS NULL THEN 0 ELSE 1 END, CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, expires_at FROM _kora_magic_link").
		WithArgs("test.local", hashMagicLinkToken("fixed-magic-token")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "used_flag", "revoked_flag", "expires_at"}).
			AddRow("ml-1", "john@test.com", 0, 0, time.Now().Add(time.Hour)))
	mock.ExpectExec("UPDATE _kora_magic_link SET used_at = \\? WHERE id = \\? AND used_at IS NULL").
		WithArgs(sqlmock.AnyArg(), "ml-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT name, email, full_name, enabled, COALESCE\\(roles, ''\\) FROM _kora_user WHERE site = \\? AND email = \\?").
		WithArgs("test.local", "john@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"name", "email", "full_name", "enabled", "roles"}).
			AddRow("john", "john@test.com", "John Doe", true, "Administrator"))
	mock.ExpectExec("UPDATE _kora_user\\s+SET email_verified_at = COALESCE\\(email_verified_at, \\?\\), modified = CURRENT_TIMESTAMP\\s+WHERE site = \\? AND email = \\?").
		WithArgs(sqlmock.AnyArg(), "test.local", "john@test.com").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO _kora_session \\(sid, site, user, data, expires_at, created_at\\)").
		WithArgs(sqlmock.AnyArg(), "test.local", "john", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req = httptest.NewRequest(http.MethodPost, "/api/auth/magic-link/verify", strings.NewReader(`{"token":"fixed-magic-token"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var verifyResp struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("unmarshal verify response: %v", err)
	}
	if verifyResp.SID == "" {
		t.Fatal("verify response did not include sid")
	}

	mock.ExpectQuery("SELECT data, expires_at FROM _kora_session WHERE site = \\? AND sid = \\?").
		WithArgs("test.local", verifyResp.SID).
		WillReturnRows(sessionRows)
	mock.ExpectQuery("SELECT id, email, created_at, expires_at, used_at, revoked_at FROM _kora_magic_link").
		WithArgs("test.local", "john@test.com", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(magicLinkRows)

	req = httptest.NewRequest(http.MethodGet, "/api/auth/magic-links", nil)
	req.AddCookie(&http.Cookie{Name: "kora_sid", Value: verifyResp.SID})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	mock.ExpectExec("UPDATE _kora_magic_link\\s+SET revoked_at = COALESCE\\(revoked_at, \\?\\)\\s+WHERE site = \\? AND email = \\? AND id = \\?").
		WithArgs(sqlmock.AnyArg(), "test.local", "john@test.com", "ml-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req = httptest.NewRequest(http.MethodPost, "/api/auth/magic-links/ml-1/revoke", nil)
	req.AddCookie(&http.Cookie{Name: "kora_sid", Value: verifyResp.SID})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
