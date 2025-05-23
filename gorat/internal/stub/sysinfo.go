package stub

import (
	"log"
	"net"
	"os"
	"os/user"
	"runtime"
)

// SystemInfo holds basic system information of the stub.
type SystemInfo struct {
	OS       string `json:"os"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip,omitempty"` // Local IP address (best effort)
	Username string `json:"username"`
}

// GetSystemInfo gathers basic system information.
func GetSystemInfo() SystemInfo {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Error getting hostname: %v", err)
		hostname = "unknown"
	}

	currentUser, err := user.Current()
	var username string
	if err != nil {
		log.Printf("Error getting current user: %v", err)
		username = "unknown"
	} else {
		username = currentUser.Username
	}

	// Attempt to get a non-loopback local IP address
	var localIP string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting interface addresses: %v", err)
	} else {
		for _, address := range addrs {
			// Check the address type and if it is not a loopback
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil { // We prefer IPv4
					localIP = ipnet.IP.String()
					break
				}
			}
		}
	}
	if localIP == "" {
		log.Println("Could not determine a non-loopback local IP address.")
	}


	return SystemInfo{
		OS:       runtime.GOOS,
		Hostname: hostname,
		Username: username,
		IP:       localIP,
	}
}
