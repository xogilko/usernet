package manifest

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ResponseTemplate represents a template for a specific content type
type ResponseTemplate struct {
	ContentType  string             `json:"content_type"`
	Template     string             `json:"template,omitempty"`
	TemplateFile string             `json:"template_file,omitempty"`
	compiled     *template.Template // cached compiled template
}

// ServiceManifest represents the manifest data for a service URL
type ServiceManifest struct {
	DefaultResponse json.RawMessage            `json:"default_response"`
	UserAgentCases  map[string]json.RawMessage `json:"user_agent_cases"`
	CountryCases    map[string]json.RawMessage `json:"country_cases"`
	Templates       []ResponseTemplate         `json:"templates,omitempty"`
}

// RequestContext holds all the information about an incoming request
type RequestContext struct {
	UserAgent     string
	AcceptTypes   []string
	Headers       map[string][]string
	PreferredType string // The content type we'll respond with
	Country       string // The country code for the request
}

// ManifestManager handles the storage and retrieval of service manifests
type ManifestManager struct {
	manifests map[string]*ServiceManifest
	mu        sync.RWMutex
	basePath  string
}

// NewManifestManager creates a new manifest manager
func NewManifestManager(basePath string) *ManifestManager {
	return &ManifestManager{
		manifests: make(map[string]*ServiceManifest),
		basePath:  basePath,
	}
}

func sanitizeFilename(serviceURL string) string {
	serviceURL = strings.TrimPrefix(serviceURL, "https://")
	serviceURL = strings.TrimPrefix(serviceURL, "http://")
	serviceURL = strings.ReplaceAll(serviceURL, "/", "_")
	serviceURL = strings.ReplaceAll(serviceURL, ":", "_")
	return serviceURL
}

// LoadManifest loads a manifest for a service URL
func (m *ManifestManager) LoadManifest(serviceURL string) (*ServiceManifest, error) {
	// First try to read from cache
	m.mu.RLock()
	if manifest, exists := m.manifests[serviceURL]; exists {
		m.mu.RUnlock()
		return manifest, nil
	}
	m.mu.RUnlock()

	// If not in cache, load from file
	manifestPath := filepath.Join(m.basePath, sanitizeFilename(serviceURL)+".json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default manifest if file doesn't exist
			return &ServiceManifest{
				DefaultResponse: json.RawMessage(`{"message": "No manifest available for this service"}`),
				UserAgentCases:  make(map[string]json.RawMessage),
				CountryCases:    make(map[string]json.RawMessage),
			}, nil
		}
		return nil, err
	}

	var manifest ServiceManifest
	fmt.Printf("DEBUG: Raw JSON data length: %d bytes\n", len(data))
	fmt.Printf("DEBUG: Raw JSON data: %s\n", string(data))

	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Printf("DEBUG: JSON unmarshal error: %v\n", err)
		return nil, err
	}

	// Debug: Print what was loaded
	fmt.Printf("DEBUG: Loaded manifest for %s with %d templates\n", serviceURL, len(manifest.Templates))
	for i, tmpl := range manifest.Templates {
		fmt.Printf("DEBUG: Template %d: ContentType=%s, TemplateFile=%s\n", i, tmpl.ContentType, tmpl.TemplateFile)
	}

	// Store in cache with write lock
	m.mu.Lock()
	m.manifests[serviceURL] = &manifest
	m.mu.Unlock()

	return &manifest, nil
}

// determineResponseType analyzes Accept headers to choose response type
func (m *ManifestManager) determineResponseType(accept []string) string {
	// Default to JSON if no Accept header
	if len(accept) == 0 {
		return "application/json"
	}

	// Parse and sort Accept header by q value
	for _, typ := range accept {
		if strings.Contains(typ, "text/html") {
			return "text/html"
		}
		if strings.Contains(typ, "text/plain") {
			return "text/plain"
		}
		if strings.Contains(typ, "application/json") {
			return "application/json"
		}
	}

	return "application/json" // default fallback
}

// urlize converts a string to a URL-friendly format
func urlize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// GetResponseForRequest gets the appropriate response for a given request context
func (m *ManifestManager) GetResponseForRequest(serviceURL string, ctx *RequestContext) (interface{}, string, error) {
	fmt.Printf("Loading manifest for: %s\n", serviceURL)
	manifest, err := m.LoadManifest(serviceURL)
	if err != nil {
		fmt.Printf("Error loading manifest: %v\n", err)
		return nil, "", err
	}

	// Determine content type
	responseType := m.determineResponseType(ctx.AcceptTypes)
	ctx.PreferredType = responseType
	fmt.Printf("Response type determined as: %s\n", responseType)

	// Get raw JSON response based on user agent and country
	fmt.Printf("Matching request context for user agent: %s\n", ctx.UserAgent)
	rawResponse, err := m.matchRequestContext(manifest, ctx)
	if err != nil {
		fmt.Printf("Error matching request context: %v\n", err)
		return nil, "", err
	}

	// If JSON is requested, return as is
	if responseType == "application/json" {
		fmt.Printf("Returning JSON response\n")
		return rawResponse, responseType, nil
	}

	// For other types, find and apply appropriate template
	fmt.Printf("Looking for template with content type: %s\n", responseType)
	for i, tmpl := range manifest.Templates {
		if tmpl.ContentType == responseType {
			fmt.Printf("Found matching template\n")

			// Create a copy of the template to work with
			tmplCopy := tmpl

			if tmplCopy.compiled == nil {
				fmt.Printf("Template not compiled, compiling now\n")
				// Create template with custom functions
				funcMap := template.FuncMap{
					"urlize": urlize,
				}

				// Get template content
				var templateContent string
				if tmplCopy.TemplateFile != "" {
					// Load template from file
					templatePath := filepath.Join(m.basePath, tmplCopy.TemplateFile)
					fmt.Printf("Loading template from file: %s\n", templatePath)
					content, err := os.ReadFile(templatePath)
					if err != nil {
						fmt.Printf("Error reading template file: %v\n", err)
						return nil, "", fmt.Errorf("template file error: %v", err)
					}
					templateContent = string(content)
				} else {
					templateContent = tmplCopy.Template
				}

				// Compile template
				fmt.Printf("Compiling template\n")
				compiled, err := template.New("response").Funcs(funcMap).Parse(templateContent)
				if err != nil {
					fmt.Printf("Error compiling template: %v\n", err)
					return nil, "", fmt.Errorf("template compilation error: %v", err)
				}

				// Store compiled template in the manifest with proper locking
				m.mu.Lock()
				manifest.Templates[i].compiled = compiled
				m.mu.Unlock()

				tmplCopy.compiled = compiled
			}

			// Execute template with JSON data
			fmt.Printf("Executing template\n")
			var data interface{}
			if err := json.Unmarshal(rawResponse, &data); err != nil {
				fmt.Printf("Error unmarshaling JSON: %v\n", err)
				return nil, "", err
			}

			var buf strings.Builder
			if err := tmplCopy.compiled.Execute(&buf, data); err != nil {
				fmt.Printf("Error executing template: %v\n", err)
				return nil, "", err
			}

			fmt.Printf("Template execution successful\n")
			return buf.String(), responseType, nil
		}
	}

	fmt.Printf("No matching template found, returning plain text %s\n", manifest.Templates)
	// If no template found, convert to string representation
	return string(rawResponse), "text/plain", nil
}

// matchRequestContext matches a request context against manifest cases and merges responses
func (m *ManifestManager) matchRequestContext(manifest *ServiceManifest, ctx *RequestContext) (json.RawMessage, error) {
	// Start with default response
	response := manifest.DefaultResponse

	// If country is specified, try to merge country-specific response
	if ctx.Country != "" {
		if countryResponse, exists := manifest.CountryCases[ctx.Country]; exists {
			// Merge country response with default response
			merged, err := m.mergeResponses(response, countryResponse)
			if err != nil {
				return nil, err
			}
			response = merged
		}
	}

	// If user agent is specified, try to merge user agent specific response
	if ctx.UserAgent != "" {
		for uaPattern, uaResponse := range manifest.UserAgentCases {
			if strings.Contains(ctx.UserAgent, uaPattern) {
				// Merge user agent response with current response
				merged, err := m.mergeResponses(response, uaResponse)
				if err != nil {
					return nil, err
				}
				response = merged
				break
			}
		}
	}

	return response, nil
}

// mergeResponses merges two JSON responses, with the second response taking precedence
func (m *ManifestManager) mergeResponses(defaultResp, overrideResp json.RawMessage) (json.RawMessage, error) {
	var defaultMap, overrideMap map[string]interface{}

	if err := json.Unmarshal(defaultResp, &defaultMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(overrideResp, &overrideMap); err != nil {
		return nil, err
	}

	// Merge maps
	for k, v := range overrideMap {
		defaultMap[k] = v
	}

	// Convert back to JSON
	return json.Marshal(defaultMap)
}

// GetResponseForUserAgent gets the appropriate response for a given user agent
func (m *ManifestManager) GetResponseForUserAgent(serviceURL, userAgent string) (json.RawMessage, error) {
	manifest, err := m.LoadManifest(serviceURL)
	if err != nil {
		return nil, err
	}

	ctx := &RequestContext{
		UserAgent: userAgent,
	}

	return m.matchRequestContext(manifest, ctx)
}

// Helper function to get map keys as a slice
func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// UpdateManifest updates the manifest for a service URL
func (m *ManifestManager) UpdateManifest(serviceURL string, manifest *ServiceManifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Save to file
	manifestPath := filepath.Join(m.basePath, sanitizeFilename(serviceURL)+".json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return err
	}

	m.manifests[serviceURL] = manifest
	return nil
}
