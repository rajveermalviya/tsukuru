package androidbuilder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetAndroidSdkRoot() (path string, licenses bool, err error) {
	path = os.Getenv("ANDROID_SDK_ROOT")
	if path == "" {
		return "", false, errors.New("getAndroidSdkRoot: env ANDROID_SDK_ROOT not set")
	}

	licenses, err = checkAndroidSdkRoot(path)
	if err != nil {
		return "", false, fmt.Errorf("getAndroidSdkRoot: %w", err)
	}

	return
}

func checkAndroidSdkRoot(androidSdkRoot string) (licenses bool, err error) {
	entries, err := os.ReadDir(androidSdkRoot)
	if err != nil {
		return false, fmt.Errorf("checkAndroidSdkRoot: %w", err)
	}

	var hasPlatformTools, hasCmdlineTools bool

	for _, entry := range entries {
		switch entry.Name() {
		case "licenses":
			licenses = true
		case "platform-tools":
			hasPlatformTools = true
		case "cmdline-tools":
			hasCmdlineTools = true
		}
	}

	if hasPlatformTools {
		_, err = os.Stat(filepath.Join(androidSdkRoot, "platform-tools", getName("adb")))
		if err != nil {
			return false, fmt.Errorf("checkAndroidSdkRoot: %w", err)
		}
	} else {
		return false, errors.New("checkAndroidSdkRoot: unable to find \"platform-tools\" in " + androidSdkRoot)
	}

	if hasCmdlineTools {
		_, err = os.Stat(filepath.Join(androidSdkRoot, "cmdline-tools", "latest", "bin", getName("sdkmanager")))
		if err != nil {
			return false, fmt.Errorf("checkAndroidSdkRoot: %w", err)
		}
	} else {
		return false, errors.New("checkAndroidSdkRoot: unable to find \"cmdline-tools\" in " + androidSdkRoot)
	}

	return licenses, nil
}

func findAndroidBuildTools(androidSdkRoot, targetSdkVersion string) (string, error) {
	buildTools := filepath.Join(androidSdkRoot, "build-tools")
	entries, err := os.ReadDir(buildTools)
	if err != nil {
		return "", fmt.Errorf("findAndroidBuildTools: %w", err)
	}

	latestVersion := ""
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, targetSdkVersion) {
				latestVersion = name
			}
		}
	}

	if latestVersion == "" {
		return "", errors.New("findAndroidBuildTools: unable to find \"build-tools\" for targetSdkVersion=" + targetSdkVersion)
	}

	return filepath.Join(buildTools, latestVersion), nil
}

func downloadAndroidBuildtools(androidSdkRoot, targetSdkVersion string) (string, error) {
	latestVersion, err := FindLatestVersionOfSdk("build-tools", targetSdkVersion, true)
	if err != nil {
		return "", fmt.Errorf("downloadAndroidBuildtools: %w", err)
	}

	// if found install it via sdkmanager
	sdkmanager := filepath.Join(androidSdkRoot, "cmdline-tools", "latest", "bin", getName("sdkmanager"))
	cmd := exec.Command(sdkmanager, "build-tools;"+latestVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("downloadAndroidBuildtools: %w", err)
	}

	return filepath.Join(androidSdkRoot, "build-tools", latestVersion), nil
}

func checkAndroidBuildTools(buildTools string) error {
	entries, err := os.ReadDir(buildTools)
	if err != nil {
		return fmt.Errorf("checkAndroidBuildTools: %w", err)
	}

	var hasAapt2, hasD8, hasZipalign, hasApksigner bool

	for _, entry := range entries {
		if entry.Type().IsRegular() {
			switch entry.Name() {
			case getName("aapt2"):
				hasAapt2 = true
			case getName("d8"):
				hasD8 = true
			case getName("zipalign"):
				hasZipalign = true
			case getName("apksigner"):
				hasApksigner = true
			}
		}
	}

	toolsNotFound := make([]string, 0, 4)
	if !hasAapt2 {
		toolsNotFound = append(toolsNotFound, "aapt2")
	}
	if !hasD8 {
		toolsNotFound = append(toolsNotFound, "d8")
	}
	if !hasZipalign {
		toolsNotFound = append(toolsNotFound, "zipalign")
	}
	if !hasApksigner {
		toolsNotFound = append(toolsNotFound, "apksigner")
	}

	if len(toolsNotFound) > 0 {
		return errors.New("checkAndroidBuildTools: unable to find " + strings.Join(toolsNotFound, ", ") + " in " + buildTools)
	}

	return nil
}

func findAndroidPlatform(androidSdkRoot, targetSdkVersion string) (string, error) {
	platforms := filepath.Join(androidSdkRoot, "platforms")
	entries, err := os.ReadDir(platforms)
	if err != nil {
		return "", fmt.Errorf("findAndroidPlatform: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if name == "android-"+targetSdkVersion {
				return filepath.Join(platforms, name), nil
			}
		}
	}

	return "", errors.New("findAndroidPlatform: unable to find \"android-" + targetSdkVersion + "\" in " + platforms)
}

func downloadAndroidPlatform(androidSdkRoot, targetSdkVersion string) (string, error) {
	sdkmanager := filepath.Join(androidSdkRoot, "cmdline-tools", "latest", "bin", getName("sdkmanager"))
	cmd := exec.Command(sdkmanager, "platforms;android-"+targetSdkVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("downloadAndroidPlatform: %w", err)
	}

	return filepath.Join(androidSdkRoot, "platforms", "android-"+targetSdkVersion), nil
}

func checkAndroidPlatform(platformDir string) error {
	entries, err := os.ReadDir(platformDir)
	if err != nil {
		return fmt.Errorf("checkAndroidPlatform: %w", err)
	}

	for _, entry := range entries {
		if entry.Type().IsRegular() {
			if entry.Name() == "android.jar" {
				return nil
			}
		}
	}

	return errors.New("checkAndroidPlatform: unable to find \"android.jar\" in " + platformDir)
}
