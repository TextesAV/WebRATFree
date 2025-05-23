package server

import (
	"net/http"
	"os"
	"time"

	"github.com/gorilla/sessions"
)

// Store will hold the session data.
var Store *sessions.CookieStore

// Hardcoded credentials for now
const (
	adminUser = "admin"
	adminPass = "admin"
)

func InitAuth() {
	// In a production environment, use a long, random, and secret key.
	// You can generate one using `openssl rand -base64 32`
	authKey := os.Getenv("SESSION_AUTH_KEY")
	if authKey == "" {
		authKey = "a-very-secret-and-random-key-replace-it" // Default for dev
		// It's good practice to log if a default key is being used in a real app,
		// but for this exercise, we'll keep it simple.
	}
	Store = sessions.NewCookieStore([]byte(authKey))
	Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int(time.Hour * 1 / time.Second), // 1 hour
		HttpOnly: true,
		Secure:   true, // Set to true if using HTTPS (which we are)
		SameSite: http.SameSiteLaxMode,
	}
}

// LoginHandler handles the POST request to /login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, _ := Store.Get(r, "gorat-session")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == adminUser && password == adminPass {
		session.Values["authenticated"] = true
		session.Values["username"] = username
		err = session.Save(r, w)
		if err != nil {
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	} else {
		// Set a flash message for failed login attempt
		session.AddFlash("Invalid username or password", "error_message")
		err = session.Save(r, w)
		if err != nil {
			// log error but still redirect
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// LogoutHandler handles the GET request to /logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := Store.Get(r, "gorat-session")
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1 // Delete the cookie
	err := session.Save(r, w)
	if err != nil {
		http.Error(w, "Failed to save session for logout", http.StatusInternalServerError)
		// Still redirect even if save fails
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// AuthMiddleware protects routes that require authentication.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := Store.Get(r, "gorat-session")
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	}
}
