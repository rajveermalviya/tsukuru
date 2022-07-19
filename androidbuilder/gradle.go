package androidbuilder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type GradleBuilder struct{}

func NewGradleBuilder() (*GradleBuilder, error) {
	return &GradleBuilder{}, nil
}

type gradleBuildApkOptions struct {
	androidDir string

	release bool
}

type GradleBuildApkOption func(*gradleBuildApkOptions)

func GradleBuilderOptRelease() GradleBuildApkOption {
	return func(opts *gradleBuildApkOptions) {
		opts.release = true
	}
}

func (b *GradleBuilder) BuildApk(androidDir string, opts ...GradleBuildApkOption) (string, error) {
	if filepath.Clean(androidDir) == "." {
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}

		androidDir = dir
	}

	options := &gradleBuildApkOptions{
		androidDir: androidDir,
	}
	for _, opt := range opts {
		opt(options)
	}

	gradlewScript := "gradlew"
	if runtime.GOOS == "windows" {
		gradlewScript = "gradlew.bat"
	}
	gradlew := filepath.Join(androidDir, gradlewScript)

	_, err := os.Stat(gradlew)
	if err != nil {
		return "", err
	}

	command := "assembleDebug"
	if options.release {
		command = "assembleRelease"
	}

	cmd := exec.Command(gradlew, command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = androidDir
	fmt.Println(cmd.String())
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	if !options.release {
		return filepath.Join(androidDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk"), nil
	}

	releaseUnsigned := filepath.Join(androidDir, "app", "build", "outputs", "apk", "release", "app-release-unsigned.apk")
	_, err = os.Stat(releaseUnsigned)
	if err == nil {
		return releaseUnsigned, nil
	}

	releaseSigned := filepath.Join(androidDir, "app", "build", "outputs", "apk", "release", "app-release.apk")
	_, err = os.Stat(releaseSigned)
	if err == nil {
		return releaseSigned, nil
	}

	return "", errors.New("unable to find build apk")
}

func (b *GradleBuilder) BuildAppbundle(androidDir string, opts ...GradleBuildApkOption) (string, error) {
	if filepath.Clean(androidDir) == "." {
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}

		androidDir = dir
	}

	options := &gradleBuildApkOptions{
		androidDir: androidDir,
	}
	for _, opt := range opts {
		opt(options)
	}

	gradlewScript := "gradlew"
	if runtime.GOOS == "windows" {
		gradlewScript = "gradlew.bat"
	}
	gradlew := filepath.Join(androidDir, gradlewScript)

	_, err := os.Stat(gradlew)
	if err != nil {
		return "", err
	}

	command := "bundleDebug"
	if options.release {
		command = "bundleRelease"
	}

	cmd := exec.Command(gradlew, command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = androidDir
	fmt.Println(cmd.String())
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	if options.release {
		return filepath.Join(androidDir, "app", "build", "outputs", "bundle", "release", "app-release.aab"), nil
	}

	return filepath.Join(androidDir, "app", "build", "outputs", "bundle", "debug", "app-debug.aab"), nil
}
