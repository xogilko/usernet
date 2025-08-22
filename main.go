package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"usernet/manifest"
)

//go:embed static
var staticFiles embed.FS
var manifestManager *manifest.ManifestManager

func displaySplash() {
	fmt.Print(`
╔═════════════════════════════════════════════════════════════════════╗
║                      USERNET API TERMINAL v1.0                      ║
║                                                                     ║
║     * . *  .  *       .    *    .     *   .    .  *    .   *   .    ║ 
║  .    *     .'  *    .    .  *    .    *   .   *   .   *    .    *  ║
║    .    * .'    .  *    .    *  .    .   *   .   *    .    *     .  ║
║  *   .   /   .   *   .    *     .  *    .    *    .  *    .   *   . ║
║    .  * /  *    .   *    .   *    .   *    .    *    .   *   .    * ║
║  *   . /    .  *    .  *     .   *   .   *    .   *    .    *    .  ║
║    .  *     *    .    *   .    *    .    *   .   *   .    *     .   ║
║  *    .   *   .    *    .   THANK U    *    .  *    .   *   .    *  ║
╚═════════════════════════════════════════════════════════════════════╝
`)
}

func terminalInterface() {
	for {
		fmt.Println("1. option 1")
		fmt.Println("4. Exit")
		fmt.Print("\nSelect option: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			fmt.Println("\noption 1 does nothing")
		case "4":
			return
		default:
			fmt.Println("Invalid option")
		}
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
		fmt.Printf("🏠 Returning _default manifest\n")
		response, contentType, err := manifestManager.GetResponseForRequest("_default", ctx)
		if err != nil {
			fmt.Printf("❌ Error loading _default manifest: %v\n", err)
			http.Error(w, "Error loading _default manifest", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if strResponse, ok := response.(string); ok {
			w.Write([]byte(strResponse))
			fmt.Printf("✅ _default manifest served successfully\n")
		} else {
			fmt.Printf("❌ Invalid response type for _default manifest\n")
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
func seed() ([]byte, error) {

	return []byte("seeded"), nil
}

func main() {
	displaySplash()
	go terminalInterface()
	manifestManager = manifest.NewManifestManager("manifest")
	mux := http.NewServeMux()
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//general request handling
		parts := strings.Split(r.URL.Path, "/")

		handleManifestRequest(w, r, parts[1:])
		//root request
		seed, err := seed()
		if err != nil {
			fmt.Printf("failed to seed")
			http.Error(w, "failed to seed", http.StatusNotFound)
			return
		}
		w.Write(seed)
	})
	//
	handler := enableCORS(mux)
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
