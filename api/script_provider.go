package api

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/orm"
	"github.com/asenawritescode/kora/script"
	"github.com/asenawritescode/kora/secret"
)

// scriptProvider bridges the JS runtime to the Kora engine.
// It has full access to the ORM, registry, and secrets.
type scriptProvider struct {
	tx          *orm.TxManager
	registry    *doctype.Registry
	site        string
	secretStore *secret.Store

	// HTTP allowlist controls which domains scripts can call.
	HTTPAllowlist []string

	httpClient *http.Client
}

// NewScriptProvider creates a provider with a scoped HTTP client.
func NewScriptProvider(tx *orm.TxManager, registry *doctype.Registry, site string, secretStore *secret.Store, httpAllowlist []string) script.KoraProvider {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:    10,
		IdleConnTimeout: 60 * time.Second,
	}
	return &scriptProvider{
		tx:            tx,
		registry:      registry,
		site:          site,
		secretStore:   secretStore,
		HTTPAllowlist: httpAllowlist,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// GetDoc fetches a single document by doctype and name.
func (p *scriptProvider) GetDoc(doctypeName, name string) (map[string]any, error) {
	dt := p.registry.Get(doctypeName)
	if dt == nil {
		return nil, fmt.Errorf("doctype %q not found", doctypeName)
	}
	doc, err := p.tx.GetDoc(dt, name, "")
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	return doc.ToMap(), nil
}

// GetList fetches documents with optional filters, ordering, and pagination.
func (p *scriptProvider) GetList(doctypeName string, filters map[string]any, orderBy string, limit, offset int) ([]map[string]any, error) {
	dt := p.registry.Get(doctypeName)
	if dt == nil {
		return nil, fmt.Errorf("doctype %q not found", doctypeName)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500 // cap to prevent memory exhaustion
	}

	// Convert map filters to JSON filter string (Kora ORM format).
	filterStr := mapToFilterString(filters)

	docs, _, err := p.tx.GetList(dt, filterStr, orderBy, limit, offset, "")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(docs))
	for i, doc := range docs {
		result[i] = doc.ToMap()
	}
	return result, nil
}

// mapToFilterString converts a map of field→value pairs to JSON filter format.
// Example: {customer: "CUST-0001", status: "Open"} → [["customer","=","CUST-0001"],["status","=","Open"]]
func mapToFilterString(filters map[string]any) string {
	if len(filters) == 0 {
		return ""
	}
	var parts []string
	for k, v := range filters {
		// Support operators via special map keys: {"status": ["!=", "Completed"]}
		op := "="
		val := v
		if arr, ok := v.([]any); ok && len(arr) == 2 {
			if opStr, ok2 := arr[0].(string); ok2 {
				op = opStr
			}
			val = arr[1]
		}
		valStr := fmt.Sprintf("%v", val)
		parts = append(parts, fmt.Sprintf(`["%s","%s","%s"]`, k, op, valStr))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// SaveDoc updates an existing document.
func (p *scriptProvider) SaveDoc(doctypeName string, doc map[string]any, modifiedBy string) error {
	dt := p.registry.Get(doctypeName)
	if dt == nil {
		return fmt.Errorf("doctype %q not found", doctypeName)
	}
	name, ok := doc["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("document must have a 'name' field")
	}

	// Fetch existing document to get old state.
	existing, err := p.tx.GetDoc(dt, name, "")
	if err != nil {
		return fmt.Errorf("fetching existing: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("document %q not found", name)
	}
	oldDoc := cloneScriptDocument(existing)

	doc = normalizeScriptDocumentInput(dt, p.registry, doc)

	// Merge changes into existing document.
	for k, v := range doc {
		if k != "name" && k != "creation" && k != "modified" && k != "modified_by" && k != "owner" && k != "doc_status" {
			existing.Set(k, v)
		}
	}

	return p.tx.Save(dt, existing, modifiedBy, "", oldDoc)
}

// CreateDoc creates a new document.
func (p *scriptProvider) CreateDoc(doctypeName string, doc map[string]any, owner, modifiedBy string) (map[string]any, error) {
	dt := p.registry.Get(doctypeName)
	if dt == nil {
		return nil, fmt.Errorf("doctype %q not found", doctypeName)
	}

	d := &doctype.Document{DocType: dt.Name, Fields: normalizeScriptDocumentInput(dt, p.registry, doc), IsNew: true}
	if err := p.tx.Insert(dt, d, owner, modifiedBy); err != nil {
		return nil, err
	}
	return d.ToMap(), nil
}

// DeleteDoc deletes a document by doctype and name.
func (p *scriptProvider) DeleteDoc(doctypeName, name string) error {
	dt := p.registry.Get(doctypeName)
	if dt == nil {
		return fmt.Errorf("doctype %q not found", doctypeName)
	}
	return p.tx.Delete(dt, name, "")
}

// GetSecret returns the decrypted value of a secret from _kora_secret.
func (p *scriptProvider) GetSecret(key string) (string, error) {
	if p.secretStore == nil {
		return "", fmt.Errorf("secret store not available")
	}
	return p.secretStore.Get(p.site, key)
}

// DoHTTP executes an external HTTP request with domain allowlist enforcement.
func (p *scriptProvider) DoHTTP(req *script.HTTPRequest) (*script.HTTPResponse, error) {
	if err := p.checkHTTPAllowlist(req.URL); err != nil {
		return nil, err
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("Content-Type") == "" && req.Body != "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return nil, fmt.Errorf("http: reading response: %w", err)
	}

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return &script.HTTPResponse{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    respHeaders,
		Body:       respBody,
	}, nil
}

// checkHTTPAllowlist validates that the URL's host is in the domain allowlist.
func (p *scriptProvider) checkHTTPAllowlist(urlStr string) error {
	if len(p.HTTPAllowlist) == 0 {
		return fmt.Errorf("http: external requests are disabled (no domain allowlist configured)")
	}

	// Strip scheme.
	host := urlStr
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	// Strip path.
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	// Strip port.
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)

	// Block private IPs.
	if isPrivateHost(host) {
		return fmt.Errorf("http: requests to private/internal hosts are not allowed")
	}

	// Check against allowlist.
	for _, allowed := range p.HTTPAllowlist {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "*" {
			return nil
		}
		// Exact match.
		if host == allowed {
			return nil
		}
		// Wildcard: *.safaricom.co.ke matches api.safaricom.co.ke.
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // .safaricom.co.ke
			if strings.HasSuffix(host, suffix) {
				return nil
			}
		}
	}

	return fmt.Errorf("http: domain %q is not in the allowed list", host)
}

// isPrivateHost checks if a hostname resolves to or matches a private IP range.
func isPrivateHost(host string) bool {
	// Check hostname patterns.
	privSuffixes := []string{
		".local", ".internal", ".localhost",
	}
	for _, s := range privSuffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	// Check IP ranges.
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return false
}

func normalizeScriptDocumentInput(dt *doctype.DocType, registry *doctype.Registry, fields map[string]any) map[string]any {
	if dt == nil || fields == nil {
		return fields
	}
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	for _, field := range dt.TableFields() {
		value, ok := out[field.Fieldname]
		if !ok || value == nil {
			continue
		}
		childDT := registry.Get(field.Options)
		if childDT == nil {
			continue
		}
		children, ok := scriptValueToChildDocuments(value, childDT, registry)
		if ok {
			out[field.Fieldname] = children
		}
	}
	return out
}

func scriptValueToChildDocuments(value any, childDT *doctype.DocType, registry *doctype.Registry) ([]*doctype.Document, bool) {
	switch rows := value.(type) {
	case []*doctype.Document:
		return rows, true
	case []map[string]any:
		children := make([]*doctype.Document, 0, len(rows))
		for _, row := range rows {
			children = append(children, scriptMapToDocument(row, childDT))
		}
		return children, true
	case []any:
		children := make([]*doctype.Document, 0, len(rows))
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			children = append(children, scriptMapToDocument(row, childDT))
		}
		return children, true
	default:
		return nil, false
	}
}

func scriptMapToDocument(row map[string]any, dt *doctype.DocType) *doctype.Document {
	child := doctype.NewDocument(dt.Name)
	if name, ok := row["name"].(string); ok {
		child.Name = name
		child.IsNew = name == ""
	}
	if status, ok := row["doc_status"]; ok {
		child.DocStatus = intFromScriptValue(status)
	}
	for key, value := range row {
		if key == "name" || key == "doc_status" || key == "owner" || key == "creation" || key == "modified" || key == "modified_by" {
			continue
		}
		child.Set(key, value)
	}
	return child
}

func intFromScriptValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func cloneScriptDocument(doc *doctype.Document) *doctype.Document {
	if doc == nil {
		return nil
	}
	clone := &doctype.Document{
		DocType:   doc.DocType,
		Name:      doc.Name,
		Fields:    make(map[string]any, len(doc.Fields)),
		IsNew:     doc.IsNew,
		DocStatus: doc.DocStatus,
	}
	for key, value := range doc.Fields {
		clone.Fields[key] = cloneScriptValue(value)
	}
	return clone
}

func cloneScriptValue(value any) any {
	switch v := value.(type) {
	case []*doctype.Document:
		children := make([]*doctype.Document, len(v))
		for i, child := range v {
			children[i] = cloneScriptDocument(child)
		}
		return children
	case []any:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = cloneScriptValue(item)
		}
		return items
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[key] = cloneScriptValue(item)
		}
		return m
	default:
		return value
	}
}
