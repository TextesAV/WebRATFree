//go:build windows
// +build windows

package stub

import (
	"fmt"
	"log"
	"os"
	// "path/filepath" // Not directly used here, but often useful with appPath
	"strings"

	"golang.org/x/sys/windows/registry"
)

// IsAdmin checks if the current process is running with administrator privileges.
func IsAdmin() (bool, error) {
	// This is a common heuristic: only admins can typically open this raw device.
	// It might not be 100% reliable or could be flagged by some AV/EDR.
	// A more robust method involves checking the user's token (e.g., checking for Administrators SID).
	// For simplicity in this context, we use this method and acknowledge its limitations.
	f, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		if os.IsPermission(err) {
			log.Println("Admin check: Permission denied to open \\\\.\\PHYSICALDRIVE0, likely not admin.")
			return false, nil
		}
		// Some other error occurred, not necessarily permission-related.
		log.Printf("Admin check: Error opening \\\\.\\PHYSICALDRIVE0: %v", err)
		return false, fmt.Errorf("error checking admin status via PHYSICALDRIVE0: %w", err)
	}
	// If open succeeded, we likely have admin rights.
	f.Close()
	log.Println("Admin check: Successfully opened \\\\.\\PHYSICALDRIVE0, likely admin.")
	return true, nil
}

// SetupAutostart configures the stub to run at Windows startup by adding an entry
// to the HKEY_LOCAL_MACHINE Run key. This requires administrator privileges.
// appPath should be the full, absolute path to the executable.
// appName is the desired name for the registry entry.
func SetupAutostart(appPath string, appName string) error {
	if appPath == "" {
		return fmt.Errorf("appPath cannot be empty for autostart setup")
	}
	// Basic sanitization for appName to avoid issues with registry path or command interpretation.
	// Users should ensure appName is a simple string.
	safeAppName := strings.ReplaceAll(appName, "\"", "") // Remove quotes
	safeAppName = strings.ReplaceAll(safeAppName, ";", "") // Remove semicolons
	// Further sanitization might be needed depending on how appName is formed.

	// Path to the HKLM Run key
	const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

	// Open the key with write access. This will fail if not running as admin.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open HKLM Run key ('%s'): %w. Ensure running as admin", runKeyPath, err)
	}
	defer key.Close()

	// The path to the executable should be quoted if it contains spaces.
	// The registry API itself or how CMD processes it might handle unquoted paths with spaces,
	// but it's best practice to quote them to avoid ambiguity.
	quotedAppPath := fmt.Sprintf("\"%s\"", appPath)

	// Check if the value already exists and is correct
	existingPath, _, err := key.GetStringValue(safeAppName)
	if err == nil { // Value exists
		if existingPath == quotedAppPath {
			log.Printf("Autostart entry '%s' already exists with the correct path: %s", safeAppName, quotedAppPath)
			return nil
		}
		log.Printf("Autostart entry '%s' exists with a different path ('%s'). It will be updated.", safeAppName, existingPath)
	} else if err != registry.ErrNotExist { // Some other error occurred reading the value
		log.Printf("Error checking existing autostart entry '%s': %v", safeAppName, err)
		// Proceed to set it anyway, as the state is unknown or problematic.
	}

	// Set (or overwrite) the string value in the registry.
	err = key.SetStringValue(safeAppName, quotedAppPath)
	if err != nil {
		return fmt.Errorf("failed to set autostart registry value for '%s' to '%s': %w", safeAppName, quotedAppPath, err)
	}

	log.Printf("Autostart configured successfully: '%s' -> '%s' in HKLM Run key.", safeAppName, quotedAppPath)
	return nil
}
