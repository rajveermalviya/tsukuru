package androidbuilder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func HasNdk(androidSdkRoot string) bool {
	entries, err := os.ReadDir(androidSdkRoot)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == "ndk" {
			return true
		}
	}

	return false
}

func FindLatestVersionOfNdkInstalled(androidSdkRoot string) string {
	entries, err := os.ReadDir(filepath.Join(androidSdkRoot, "ndk"))
	if err != nil {
		return ""
	}

	if len(entries) == 0 {
		return ""
	}

	entry := entries[len(entries)-1]
	dir := entry.Name()

	return filepath.Join(androidSdkRoot, "ndk", dir)
}

// ndkVersion should be "major.minor.micro" not "ndk;major.minor.micro"
func DownloadNdk(androidSdkRoot, version string) error {
	sdkmanager := filepath.Join(androidSdkRoot, "cmdline-tools", "latest", "bin", getName("sdkmanager"))
	cmd := exec.Command(sdkmanager, "ndk;"+version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("DownloadNdk: %w", err)
	}

	return nil
}
