package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type Config map[string]string

type AppError struct{}

func (m *AppError) Error() string {
	return "boom"
}

const (
	configFile     = "php-runner.yaml"
	versionFile    = ".php-version"
	defaultVersion = "8.2"
)

func fileExist(path string) bool {
	// os.Stat returns file info and an error if any.
	info, err := os.Stat(path)

	// If os.Stat returns an error, we check if it's a "not exist" error.
	// This is the standard way to check for a non-existent file.
	if os.IsNotExist(err) {
		return false
	}

	// If there was no error, something exists at the path.
	// We then check if it's a directory. If it is, we return false.
	return !info.IsDir()
}

// loadConfig loads and parses the YAML-style configuration file line by line
func loadConfig(configPath string) (Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open config file: %v", err)
	}
	defer file.Close()

	config := make(Config)
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "version: path" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format on line %d: %s", lineNumber, line)
		}

		version := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])

		if version == "" || path == "" {
			return nil, fmt.Errorf("empty version or path on line %d: %s", lineNumber, line)
		}

		// Verify PHP executable exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("Warning: PHP executable not found at %s (line %d)\n", path, lineNumber)
			continue // Skip invalid entries but don't fail completely
		}

		config[version] = path
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	if len(config) == 0 {
		return nil, fmt.Errorf("no valid PHP versions found in configuration")
	}

	return config, nil
}

// getPhpVersion determines which PHP version to use
func getPhpVersion(cwd string, config Config) string {
	// Look for .php-version file in current directory and parent directories
	version := findPhpVersionFile(cwd)
	if version != "" && config[version] != "" {
		return version
	}

	// Get current PHP version from PATH
	currentVersion := getCurrentPhpVersion()
	if currentVersion != "" && config[currentVersion] != "" {
		// Create .php-version file with current version
		createPhpVersionFile(cwd, currentVersion)
		return currentVersion
	}

	// Use default version if available
	if config[defaultVersion] != "" {
		createPhpVersionFile(cwd, defaultVersion)
		return defaultVersion
	}

	// Use first available version from config
	for ver := range config {
		createPhpVersionFile(cwd, ver)
		return ver
	}

	fmt.Printf("No valid PHP version found\n")
	os.Exit(1)
	return ""
}

// findPhpVersionFile looks for .php-version file in current and parent directories
func findPhpVersionFile(startDir string) string {
	dir := startDir
	for {
		versionPath := filepath.Join(dir, versionFile)
		if content, err := ioutil.ReadFile(versionPath); err == nil {
			version := strings.TrimSpace(string(content))
			if version != "" {
				return version
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}
	return ""
}

// getCurrentPhpVersion gets the version of PHP currently in PATH
func getCurrentPhpVersion() string {
	cmd := exec.Command("php", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse PHP version from output
	// Expected format: "PHP 8.2.0 (cli) ..." or "PHP 8.2.0-dev ..."
	re := regexp.MustCompile(`PHP (\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

// createPhpVersionFile creates a .php-version file with the specified version
func createPhpVersionFile(dir, version string) {
	versionPath := filepath.Join(dir, versionFile)
	err := ioutil.WriteFile(versionPath, []byte(version+"\n"), 0644)
	if err != nil {
		fmt.Printf("Warning: Could not create %s: %v\n", versionPath, err)
	} else {
		fmt.Printf("Created %s with PHP version %s\n", versionPath, version)
	}
}

func getConfig() (Config, error) {

	// Get home directory
	homeDir, err := os.UserHomeDir()
	configPath := filepath.Join(homeDir, configFile)

	if err != nil && fileExist(configPath) {
		config, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading config from %s: %v\n", configPath, err)
			return nil, err
		}

		return config, err
	} else {
		// Get executable directory
		exePath, err := os.Executable()
		if err != nil {
			fmt.Printf("Error getting executable path: %v\n", err)
			os.Exit(1)
		}
		exeDir := filepath.Dir(exePath)

		// Load configuration
		configPath := filepath.Join(exeDir, configFile)

		config, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading config from %s: %v\n", configPath, err)
			return config, err
		}

		return config, err
	}

	return nil, err
}

func main() {

	config, err := getConfig()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Get PHP version to use
	version := getPhpVersion(cwd, config)

	// Get PHP executable path
	phpPath, exists := config[version]
	if !exists {
		fmt.Printf("PHP version %s not found in configuration\n", version)
		os.Exit(1)
	}

	// Check if PHP executable exists
	if _, err := os.Stat(phpPath); os.IsNotExist(err) {
		fmt.Printf("PHP executable not found at: %s\n", phpPath)
		os.Exit(1)
	}

	// Execute PHP with all arguments
	args := os.Args[1:] // Skip the program name
	cmd := exec.Command(phpPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		fmt.Printf("Error executing PHP: %v\n", err)
		os.Exit(1)
	}
}
