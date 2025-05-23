package stub

import (
	"log"
	"os/exec"
	"runtime"
)

// HandleRemoteShellCommand executes the given command string and returns its combined stdout and stderr.
func HandleRemoteShellCommand(commandStr string) (string, error) {
	var cmd *exec.Cmd

	if commandStr == "" {
		return "", nil // No command, no output
	}

	log.Printf("Executing remote shell command: %s", commandStr)

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", commandStr)
	} else {
		// For Linux/macOS, use /bin/sh -c
		// This is a basic approach; a more robust solution might involve
		// detecting the shell or allowing shell selection.
		cmd = exec.Command("/bin/sh", "-c", commandStr)
	}

	outputBytes, err := cmd.CombinedOutput() // Captures both stdout and stderr
	if err != nil {
		log.Printf("Error executing command '%s': %v. Output: %s", commandStr, err, string(outputBytes))
		// Return the output along with the error, as it might contain useful info
		return string(outputBytes), err
	}

	log.Printf("Command '%s' executed successfully. Output length: %d", commandStr, len(outputBytes))
	return string(outputBytes), nil
}

// HandleListDirectory lists the contents of a given directory path.
func HandleListDirectory(path string) ([]common.FileInfo, error) {
	log.Printf("Listing directory: %s", path)
	if path == "" { // Default to current directory if path is empty
		path = "." 
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error reading directory %s: %v", path, err)
		return nil, err
	}

	var files []common.FileInfo
	for _, entry := range dirEntries {
		info, err := entry.Info() // os.DirEntry.Info() returns os.FileInfo
		if err != nil {
			// Could happen if file is deleted during ReadDir, or permission issues for specific file
			log.Printf("Error getting FileInfo for entry %s in directory %s: %v", entry.Name(), path, err)
			// Optionally, include a FileInfo with an error field or just skip.
			// For now, skip problematic entries but log them.
			continue
		}

		file := common.FileInfo{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		files = append(files, file)
	}

	log.Printf("Successfully listed %d entries for directory: %s", len(files), path)
	return files, nil
}
