package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"hypernet_api/manifest"
)

//go:embed static/*
var staticFiles embed.FS

var serviceURLs = []string{
	"https://xomud.quest",
}

var manifestManager *manifest.ManifestManager

// requestLogger middleware logs all incoming requests to the terminal
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request details to terminal
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] 🔔 REQUEST RECEIVED: %s %s from %s\n",
			timestamp, r.Method, r.URL.Path, r.RemoteAddr)

		// Log additional request details
		if r.UserAgent() != "" {
			fmt.Printf("[%s] 📱 User-Agent: %s\n", timestamp, r.UserAgent())
		}
		if len(r.Header) > 0 {
			fmt.Printf("[%s] 📋 Headers: %d headers received\n", timestamp, len(r.Header))
		}

		// Call the next handler
		next.ServeHTTP(w, r)

		// Log completion
		fmt.Printf("[%s] ✅ Request completed: %s %s\n", timestamp, r.Method, r.URL.Path)
	})
}

func displaySplash() {
	fmt.Print(`
╔═════════════════════════════════════════════════════════════════════╗
║                      HYPERNET API TERMINAL v1.0                     ║
║                                                                     ║
║     * . *  .  *       .    *    .     *   .    .  *    .   *   .    ║ 
║  .    *     .'  *    .    .  *    .    *   .   *   .   *    .    *  ║
║    .    * .'    .  *    .    *  .    .   *   .   *    .    *     .  ║
║  *   .   /   .   *   .    *     .  *    .    *    .  *    .   *   . ║
║    .  * /  *    .   *    .   *    .   *    .    *    .   *   .    * ║
║  *   . /    .  *    .  *     .   *   .   *    .   *    .    *    .  ║
║    .  *     *    .    *   .    *    .    *   .   *   .    *     .   ║
║  *    .   *   .    *    .   *     .    *    .  *    .   *   .    *  ║
╚═════════════════════════════════════════════════════════════════════╝
`)
}

func terminalInterface() {
	for {
		fmt.Println("\nHYPERNET API MANAGEMENT")
		fmt.Println("1. List Service URLs")
		fmt.Println("2. Add Service URL")
		fmt.Println("3. Remove Service URL")
		fmt.Println("4. Exit")
		fmt.Print("\nSelect option: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			fmt.Println("\nCurrent Service URLs:")
			for i, url := range serviceURLs {
				fmt.Printf("%d. %s\n", i+1, url)
			}
		case "2":
			fmt.Print("Enter new service URL: ")
			var newURL string
			fmt.Scanln(&newURL)
			serviceURLs = append(serviceURLs, newURL)
			fmt.Println("URL added successfully")
		case "3":
			fmt.Print("Enter index to remove (1-", len(serviceURLs), "): ")
			var index int
			fmt.Scanln(&index)
			if index > 0 && index <= len(serviceURLs) {
				serviceURLs = append(serviceURLs[:index-1], serviceURLs[index:]...)
				fmt.Println("URL removed successfully")
			}
		case "4":
			return
		default:
			fmt.Println("Invalid option")
		}
	}
}

func seed(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("🌱 Serving root page template\n")

	rootmap := []map[string]interface{}{
		{"template_text": template.HTML(`
		<center><i>u just made a root request to our hypernet server api!</i></center>`)},
	}

	// Parse template from embedded filesystem
	tmpl := template.Must(template.ParseFS(staticFiles, "static/index.html"))
	tmpl.Execute(w, map[string]interface{}{"rootmap": rootmap})
}

func handleManifestRequest(w http.ResponseWriter, r *http.Request, parts []string) {
	fmt.Printf("📋 MANIFEST REQUEST: parts=%v, len=%d\n", parts, len(parts))

	// Create request context
	ctx := &manifest.RequestContext{
		UserAgent:   r.UserAgent(),
		AcceptTypes: r.Header["Accept"],
		Headers:     r.Header,
	}

	// If no specific service is requested, return the api manifest
	if len(parts) == 0 || parts[0] == "" {
		fmt.Printf("🏠 Returning hypernet_api manifest\n")
		response, contentType, err := manifestManager.GetResponseForRequest("hypernet_api", ctx)
		if err != nil {
			fmt.Printf("❌ Error loading hypernet_api manifest: %v\n", err)
			http.Error(w, "Error loading hypernet_api manifest", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if strResponse, ok := response.(string); ok {
			w.Write([]byte(strResponse))
			fmt.Printf("✅ hypernet_api manifest served successfully\n")
		} else {
			fmt.Printf("❌ Invalid response type for hypernet_api manifest\n")
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
		}
		return
	}

	// Handle service-specific manifest requests
	serviceName := parts[0]
	fmt.Printf("🔧 Service manifest requested: '%s'\n", serviceName)
	response, contentType, err := manifestManager.GetResponseForRequest(serviceName, ctx)
	if err != nil {
		fmt.Printf("❌ Error loading service manifest '%s': %v\n", serviceName, err)
		http.Error(w, "Error loading service manifest", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if strResponse, ok := response.(string); ok {
		w.Write([]byte(strResponse))
		fmt.Printf("✅ Service manifest '%s' served successfully\n", serviceName)
	} else {
		fmt.Printf("❌ Invalid response type for service manifest '%s'\n", serviceName)
		http.Error(w, "Invalid response type", http.StatusInternalServerError)
	}
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		log.Printf("CORS request from origin: %s", "hidden")
		if origin != "" && w.Header().Get("Access-Control-Allow-Origin") == "" {
			// Set the Access-Control-Allow-Origin header to the request's origin
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		// Log headers for debugging
		log.Printf("Response Headers: %v", w.Header())

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type proxyResponse struct {
	body       []byte
	statusCode int
	headers    http.Header
	err        error
}

func forwardToAll(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request to forward: %s %s", r.Method, r.URL.Path)

	// Create a channel to receive responses
	responses := make(chan proxyResponse, len(serviceURLs))

	// Copy the request body for multiple reads
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Launch a goroutine for each service URL
	var wg sync.WaitGroup
	for _, serviceURL := range serviceURLs {
		wg.Add(1)
		go func(sURL string) {
			defer wg.Done()

			// Create the target URL
			targetURL := sURL + r.URL.Path
			if r.URL.RawQuery != "" {
				targetURL += "?" + r.URL.RawQuery
			}

			// Create the new request
			proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(bodyBytes))
			if err != nil {
				log.Printf("Error creating request: %v", err)
				responses <- proxyResponse{err: err}
				return
			}

			// Create a robust client
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						ServerName:         strings.TrimPrefix(sURL, "https://"),
						InsecureSkipVerify: true, // Trust the backend certificate
					},
					DisableKeepAlives:     true,
					IdleConnTimeout:       5 * time.Second,
					TLSHandshakeTimeout:   5 * time.Second,
					ResponseHeaderTimeout: 5 * time.Second,
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
				Timeout: 10 * time.Second,
			}

			// Preserve the original Host header from the client request
			originalHost := r.Host
			if originalHost == "" {
				originalHost = r.Header.Get("Host")
			}

			// Set headers for the proxied request
			proxyReq.Header = r.Header.Clone()
			proxyReq.Host = strings.TrimPrefix(sURL, "https://")
			proxyReq.Header.Set("X-Forwarded-Host", originalHost)
			proxyReq.Header.Set("X-Real-IP", strings.Split(r.RemoteAddr, ":")[0])
			proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
			proxyReq.Header.Set("X-Forwarded-Proto", "https")

			log.Printf("Forwarding request to: %s with Host: %s", targetURL, proxyReq.Host)

			resp, err := client.Do(proxyReq)
			if err != nil {
				log.Printf("Error forwarding to %s: %v", sURL, err)
				responses <- proxyResponse{err: err}
				return
			}
			defer resp.Body.Close()

			// Read the response body with timeout
			bodyC := make(chan []byte, 1)
			errC := make(chan error, 1)

			go func() {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					errC <- err
					return
				}
				bodyC <- body
			}()

			// Wait for body read with timeout
			select {
			case body := <-bodyC:
				responses <- proxyResponse{
					body:       body,
					statusCode: resp.StatusCode,
					headers:    resp.Header,
					err:        nil,
				}
			case err := <-errC:
				log.Printf("Error reading response from %s: %v", sURL, err)
				responses <- proxyResponse{err: err}
			case <-time.After(5 * time.Second):
				log.Printf("Timeout reading response from %s", sURL)
				responses <- proxyResponse{err: fmt.Errorf("response read timeout")}
			}
		}(serviceURL)
	}

	// Close responses channel after all goroutines complete
	go func() {
		wg.Wait()
		close(responses)
	}()

	// Return the first successful response
	var lastErr error
	for resp := range responses {
		if resp.err != nil {
			lastErr = resp.err
			log.Printf("Proxy error: %v", resp.err)
			continue
		}

		// Don't forward internal server errors
		if resp.statusCode >= 500 {
			lastErr = fmt.Errorf("upstream server error: %d", resp.statusCode)
			continue
		}

		// Copy headers
		for k, v := range resp.headers {
			w.Header()[k] = v
		}

		// Set status code and write body
		w.WriteHeader(resp.statusCode)
		w.Write(resp.body)
		return
	}

	// If we get here, all proxies failed
	if lastErr != nil {
		log.Printf("All proxies failed, last error: %v", lastErr)
	}
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
}

func main() {
	displaySplash()
	go terminalInterface()

	// Initialize manifest manager
	manifestManager = manifest.NewManifestManager("manifest")

	mux := http.NewServeMux()

	// Static file handler
	mux.HandleFunc("/.well-known/static/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/.well-known/static/")
		fmt.Printf("📁 STATIC FILE REQUEST: %s\n", path)

		content, err := staticFiles.ReadFile("static/" + path)
		if err != nil {
			fmt.Printf("❌ Static file not found: %s\n", path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if strings.HasSuffix(path, ".gif") {
			w.Header().Set("Content-Type", "image/gif")
		} else if strings.HasSuffix(path, ".html") {
			w.Header().Set("Content-Type", "text/html")
		}
		w.Write(content)
		fmt.Printf("✅ Static file served: %s (%d bytes)\n", path, len(content))
	})

	// hypernet namespace handler
	mux.HandleFunc("/hypernet/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/hypernet/")
		parts := strings.Split(path, "/")
		fmt.Printf("🌐 HYPERNET REQUEST: %s -> parts: %v\n", path, parts)

		if len(parts) == 0 {
			fmt.Printf("❌ Invalid hypernet path\n")
			http.Error(w, "Invalid hypernet path", http.StatusBadRequest)
			return
		}

		switch parts[0] {
		case "manifest":
			handleManifestRequest(w, r, parts[1:])
		default:
			fmt.Printf("❌ Unknown hypernet endpoint: %s\n", parts[0])
			http.Error(w, "Unknown hypernet endpoint", http.StatusNotFound)
		}
	})

	// Root handler - serves initial HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Handle manifest requests by delegating to the dedicated function
		if strings.HasPrefix(r.URL.Path, "/hypernet/manifest") {
			parts := strings.Split(r.URL.Path, "/")
			// Skip /hypernet/manifest to get the remaining parts
			handleManifestRequest(w, r, parts[2:])
			return
		}

		// For root path, serve the seed template
		if r.URL.Path == "/" {
			fmt.Printf("🏠 ROOT REQUEST: serving main page\n")
		} else {
			fmt.Printf("🔍 UNKNOWN PATH: %s (falling back to root handler)\n", r.URL.Path)
		}
		seed(w, r)
	})

	// Apply middleware in order: requestLogger -> enableCORS -> mux
	handler := requestLogger(enableCORS(mux))

	log.Println("Starting server on :8080")

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
