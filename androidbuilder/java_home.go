package androidbuilder

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getJavaHome() (string, error) {
	// first try JAVA_HOME env var
	env := os.Getenv("JAVA_HOME")
	if env != "" {
		err := checkJavaHome(env)
		if err != nil {
			// fail fast if JAVA_HOME doesn't have required binaries
			return "", fmt.Errorf("getJavaHome: invalid JAVA_HOME: %w", err)
		}
		return env, nil
	}

	// fallback
	javaHome := tryFindJavaHome()
	if javaHome == "" {
		return "", errors.New("getJavaHome: unable to find JAVA_HOME")
	}

	// TODO: try jre from android studio

	return javaHome, nil
}

func tryFindJavaHome() string {
	// check if java binary is in PATH
	javaBin, err := exec.LookPath(getName("java"))
	if err != nil {
		return ""
	}

	// print jvm properties
	cmd := exec.Command(javaBin, "-XshowSettings:properties", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// parse JAVA_HOME from output
	var javaHome string

	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, "java.home = ") {
			javaHome = strings.TrimPrefix(text, "java.home = ")
			break
		}
	}

	if javaHome == "" {
		return ""
	}

	err = checkJavaHome(javaHome)
	if err != nil {
		return ""
	}

	return javaHome
}

func checkJavaHome(javaHome string) error {
	bin := filepath.Join(javaHome, "bin")
	entries, err := os.ReadDir(bin)
	if err != nil {
		return fmt.Errorf("checkJavaHome: %w", err)
	}

	var hasJava, hasJavac, hasJar, hasKeytool bool

	for _, entry := range entries {
		if entry.Type().IsRegular() {
			switch entry.Name() {
			case getName("java"):
				hasJava = true
			case getName("javac"):
				hasJavac = true
			case getName("jar"):
				hasJar = true
			case getName("keytool"):
				hasKeytool = true
			}
		}
	}

	toolsNotFound := make([]string, 0, 3)
	if !hasJava {
		toolsNotFound = append(toolsNotFound, "java")
	}
	if !hasJavac {
		toolsNotFound = append(toolsNotFound, "javac")
	}
	if !hasJar {
		toolsNotFound = append(toolsNotFound, "jar")
	}
	if !hasKeytool {
		toolsNotFound = append(toolsNotFound, "keytool")
	}

	if len(toolsNotFound) > 0 {
		return errors.New("checkJavaHome: unable to find " + strings.Join(toolsNotFound, ", ") + " in " + bin)
	}

	return nil
}

func findOrGenerateDebugKeystore(keytool string) (string, error) {
	keystore, err := findDebugKeystore()
	if err != nil {
		keystore, err = generateDebugKeystore(keytool)
		if err != nil {
			return "", err
		}
	}

	return keystore, nil
}

func findDebugKeystore() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("findDebugKeystore: %w", err)
	}

	debugKeystore := filepath.Join(home, ".android", "debug.keystore")
	_, err = os.Stat(debugKeystore)
	if err != nil {
		return "", fmt.Errorf("findDebugKeystore: %w", err)
	}

	return debugKeystore, nil
}

func generateDebugKeystore(keytool string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("generateDebugKeystore: %w", err)
	}

	debugKeystore := filepath.Join(home, ".android", "debug.keystore")

	cmd := exec.Command(
		keytool, "-genkey",
		"-v",
		"-keystore", debugKeystore,
		"-storepass", "android",
		"-alias", "androiddebugkey",
		"-keypass", "android",
		"-dname", "CN=Android Debug,O=Android,C=US",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		return "", fmt.Errorf("compileResources: %w", err)
	}

	return "", nil
}
