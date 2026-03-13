package o11yapiserver

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

// TenantBranding represents the white-label branding for a tenant domain.
// All fields are optional — the frontend degrades gracefully with sensible defaults.
// When Logo is empty, the frontend renders a text-only wordmark using Name.
type TenantBranding struct {
	Name         string `json:"name"`
	Logo         string `json:"logo"`
	Favicon      string `json:"favicon"`
	PrimaryColor string `json:"primaryColor"`
	OrgSlug      string `json:"orgSlug"`
	IssuerURL    string `json:"issuerUrl"`
	ClientID     string `json:"clientId"`
	ProductName  string `json:"productName"`
}

// defaultTenant is the fallback when no hostname match is found.
// No logo — text wordmark only. Monochrome accent.
var defaultTenant = &TenantBranding{
	Name:         "O11y",
	Logo:         "",
	Favicon:      "",
	PrimaryColor: "#ffffff",
	OrgSlug:      "",
	IssuerURL:    "",
	ClientID:     "",
	ProductName:  "O11y",
}

// tenantRegistry maps hostname → branding. Loaded from TENANTS env var.
var tenantRegistry map[string]*TenantBranding

func init() {
	tenantRegistry = make(map[string]*TenantBranding)

	// Load tenants from TENANTS env var (JSON map of hostname → branding)
	if raw := os.Getenv("TENANTS"); raw != "" {
		var entries map[string]*TenantBranding
		if err := json.Unmarshal([]byte(raw), &entries); err == nil {
			for k, v := range entries {
				// Ensure productName defaults
				if v.ProductName == "" {
					v.ProductName = "O11y"
				}
				tenantRegistry[k] = v
			}
		}
	}
}

func getTenantForHost(host string) *TenantBranding {
	// Strip port
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if t, ok := tenantRegistry[host]; ok {
		return t
	}

	return defaultTenant
}

func (provider *provider) addTenantRoutes(router *mux.Router) error {
	// Public endpoint — no auth required. Frontend calls this on page load to get branding.
	router.HandleFunc("/api/v1/tenant", func(w http.ResponseWriter, r *http.Request) {
		tenant := getTenantForHost(r.Host)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tenant)
	}).Methods(http.MethodGet)

	return nil
}
