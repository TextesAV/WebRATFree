//go:build windows

package stub

import (
	"log"
	"os" // For os.Executable()

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// IsAdmin checks if the current process is running with administrator privileges.
func IsAdmin() bool {
	var sid *windows.SID
	// Authority needs to be NT Authority
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2, // RID count
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		log.Printf("SID Error for admin check: %s", err)
		return false
	}
	defer windows.FreeSid(sid) // Ensure SID is freed

	// Get the current process token.
	// Using 0 for the process handle gets the current process.
	// However, windows.OpenCurrentProcessToken() is a more direct way.
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		log.Printf("OpenCurrentProcessToken Error: %s", err)
		return false
	}
	defer token.Close()


	member, err := token.IsMember(sid)
	if err != nil {
		log.Printf("Token IsMember Check Error: %s", err)
		return false
	}
	return member
}

// AddToAutostart adds the specified executable to Windows startup via the Run registry key.
// It targets HKEY_LOCAL_MACHINE, so it requires administrator privileges.
func AddToAutostart(executablePath string, entryName string) error {
	// HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Run
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
	if err != nil {
		return err // This error will indicate if permissions are insufficient or key doesn't exist (though it should)
	}
	defer key.Close()

	// Set the string value in the registry.
	// This will create the value if it doesn't exist, or overwrite it if it does.
	err = key.SetStringValue(entryName, "\""+executablePath+"\"") // Quoting the path is good practice
	if err != nil {
		return err
	}
	log.Printf("Successfully set autostart registry key '%s' to '%s'", entryName, executablePath)
	return nil
}

// GetExecutablePath returns the absolute path to the currently running executable.
func GetExecutablePath() (string, error) {
	return os.Executable()
}
