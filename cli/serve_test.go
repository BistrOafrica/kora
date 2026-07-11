package cli

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asenawritescode/kora/doctype"
	knet "github.com/asenawritescode/kora/net"
	"github.com/gin-gonic/gin"
)

func TestFallbackPathSiteRoutingStillDispatchesAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	siteRouter := knet.NewSiteRouter([]*knet.LoadedSite{
		{
			Name:     "live-demo",
			Config:   knet.SiteRouterConfig{Hostname: "live-demo"},
			DB:       &sql.DB{},
			Registry: doctype.NewRegistry(),
		},
	})
	router.Use(siteRouter.Middleware())

	// This matches the non-SPA fallback branch in runServe.
	knet.RegisterPathSiteRoutes(router, siteRouter, nil)

	router.GET("/api/test-site", func(c *gin.Context) {
		if _, ok := c.Get("site_db"); !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "site_db_missing"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"site": c.GetString("site_name")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/s/live-demo/api/test-site", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if body := w.Body.String(); body != "{\"site\":\"live-demo\"}" {
		t.Fatalf("body = %s", body)
	}
}
