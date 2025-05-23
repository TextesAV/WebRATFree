package server

import (
	"net/http"
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/gorilla/sessions"
)

var store *sessions.CookieStore

func init() {
	key := make([]byte, 64) // Generate a 64-byte key
	_, err := rand.Read(key)
	if err != nil {
		log.Fatalf("Error generating random key for cookie store: %v", err)
	}
	store = sessions.NewCookieStore(key)
	log.Printf("Cookie store initialized with key: %s\n", hex.EncodeToString(key)) // Log the key for debugging if needed, remove in production
}

// isAuthenticated checks if a user is authenticated based on session data.
func isAuthenticated(r *http.Request) bool {
	session, err := store.Get(r, "go-rat-session")
	if err != nil {
		return false // Error getting session, treat as not authenticated
	}

	auth, ok := session.Values["authenticated"].(bool)
	if !ok || !auth {
		return false
	}
	return true
}

// authMiddleware wraps handlers for protected routes.
// If not authenticated, it redirects to /login.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			log.Printf("User not authenticated, redirecting to /login from %s", r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	}
}
