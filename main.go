package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"sort"
	"golang.org/x/mod/semver"
)

type Config map[string]string

const (
	configFileName = "php-runner.map"
	versionFile    = ".php-version"
	defaultVersion = "8.2"
)

func main() {
	// Load configuration
	configPath, err := findConfigFile()
	if err != nil {
		fmt.Printf("Error finding config file: %v\n", err)
		os.Exit(1)
	}

	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config from %s: %v\n", configPath, err)
		os.Exit(1)
	}

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

// findConfigFile searches for php-runner.map in platform-specific locations
func findConfigFile() (string, error) {
	var searchPaths []string

	if runtime.GOOS == "windows" {
		// Windows paths
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			searchPaths = append(searchPaths, filepath.Join(userProfile, configFileName))
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			searchPaths = append(searchPaths, filepath.Join(appData, configFileName))
		}
		if programData := os.Getenv("PROGRAMDATA"); programData != "" {
			searchPaths = append(searchPaths, filepath.Join(programData, configFileName))
		}
	} else {
		// Unix-like systems (Linux, macOS, etc.)
		if home := os.Getenv("HOME"); home != "" {
			searchPaths = append(searchPaths, filepath.Join(home, "."+configFileName))
		}
		searchPaths = append(searchPaths, filepath.Join("/etc", configFileName))
		searchPaths = append(searchPaths, filepath.Join("/usr/local", configFileName))
	}

	// Always add executable path as last option
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchPaths = append(searchPaths, filepath.Join(exeDir, configFileName))
	}

	// Return the first existing file
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If no file found, return the first path for error messages
	if len(searchPaths) > 0 {
		return searchPaths[0], fmt.Errorf("config file not found in any of these locations:\n%s", strings.Join(searchPaths, "\n"))
	}

	return "", fmt.Errorf("could not determine config file locations")
}

// loadConfig loads and parses the configuration file line by line
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

	// Create a slice to store the keys
	versions := make([]string, 0, len(config))

	// Iterate over the map and append each key to the slice
	for key, _ := range config {
		//fmt.Printf("%s\n", key)
		versions = append(versions, key)
	}

	versions = sortVersions(versions)
	if len(versions) > 1 {
		createPhpVersionFile(cwd, versions[0])
		return versions[0]
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
		if content, err := os.ReadFile(versionPath); err == nil {
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

  // Find executable in %PATH%
	phpPath, err := exec.LookPath("php")
	if err != nil {
		return "";
	}

	// Get current app executable
  exePath, err := os.Executable();
	if err != nil {
		return "";
	}

	// Check if php.exe and php-runner are same file
	if exePath == phpPath {
		return "";
	}

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
	err := os.WriteFile(versionPath, []byte(version+"\n"), 0644)
	if err != nil {
		fmt.Printf("Warning: Could not create %s: %v\n", versionPath, err)
	} else {
		fmt.Printf("Created %s with PHP version %s\n", versionPath, version)
	}
}

// sortVersions sorts a list of version strings from highest to lowest.
// It ignores invalid versions and returns the sorted list without the "v" prefix.
func sortVersions(versions []string) []string {
	
	// 1. Filter the list to get only valid, canonical versions
	// We must store the "v" prefixed version for correct comparison.
	validVersions := []string{}
	for _, currentVersion := range versions {
		testVersion := currentVersion
		if !strings.HasPrefix(testVersion, "v") {
			testVersion = "v" + testVersion
		}

		if semver.IsValid(testVersion) {
			validVersions = append(validVersions, testVersion)
		}
	}

	// 2. Sort the slice of valid versions in-place
	// We use sort.Slice for a custom comparison.
	sort.Slice(validVersions, func(i, j int) bool {
		// We want to sort from "mayor to minor" (descending).
		// The function should return 'true' if item 'i' comes *before* item 'j'.
		// This means we return 'true' if version[i] > version[j].
		//
		// semver.Compare(v1, v2) returns:
		//   +1 if v1 > v2
		//   -1 if v1 < v2
		//    0 if v1 == v2
		return semver.Compare(validVersions[i], validVersions[j]) == 1
	})
	
	// 3. Create a new list with the "v" prefix stripped
	// (as requested in the previous step)
	result := make([]string, len(validVersions))
	for i, v := range validVersions {
		result[i] = strings.TrimPrefix(v, "v")
	}

	return result
}