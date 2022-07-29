package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"
)

func checkin(mainPackagePath string) {
	// CGO_ENABLED=1 GOOS=android go list -deps -f '{{ .Dir }}'

	cmd := exec.Command("go", "list", "-deps", "-f", "{{ .Dir }}", mainPackagePath)
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=android",
	)
	fmt.Println(cmd.String())
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(ee.Stderr)
		}
		panic(err)
	}

	dependenciesFromTsukuruFile := []string{}

	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		dir := strings.TrimSpace(s.Text())
		entries, err := os.ReadDir(dir + "/")
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == "tsukurufile" {
				path := filepath.Join(dir, name)
				tf, err := readTsukuruFile(path)
				if err != nil {
					panic("failed to parse tsukurufile at: " + path)
				}

				dependenciesFromTsukuruFile = append(dependenciesFromTsukuruFile, tf.Android.Dependencies...)
			}
		}
	}

	// TODO: handle versioning
	dependenciesFromTsukuruFile, err = deduplicate(dependenciesFromTsukuruFile)
	if err != nil {
		panic(err)
	}

	dependenciesFromBuildGradle, err := getDependenciesFromBuildGradle()
	if err != nil {
		panic(err)
	}

	for i, dep := range dependenciesFromTsukuruFile {
		if slices.Contains(dependenciesFromBuildGradle, dep) {
			dependenciesFromTsukuruFile = slices.Delete(dependenciesFromTsukuruFile, i, i+1)
		}
	}

	err = writeDependenciesToBuildGradle(dependenciesFromTsukuruFile)
	if err != nil {
		panic(err)
	}
}

type TsukuruFile struct {
	Android struct {
		Dependencies []string
	}
}

// # tsukurufile
//
//	tsukuru v1alpha
//
//	android (
//	   com.my.dependency:major.minor.patch
//	)

// extremely barebones parser
func readTsukuruFile(name string) (tsukuruFile TsukuruFile, err error) {
	f, err := os.Open(name)
	if err != nil {
		return tsukuruFile, fmt.Errorf("readTsukuruFile: %w", err)
	}
	defer f.Close()

	isAndroidBlock := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := strings.TrimSpace(s.Text())

		if !isAndroidBlock &&
			(strings.HasPrefix(l, "android (") ||
				strings.HasPrefix(l, "android(")) {
			isAndroidBlock = true
			continue
		}

		if isAndroidBlock && strings.HasPrefix(l, ")") {
			isAndroidBlock = false
		}

		if isAndroidBlock {
			l = strings.TrimPrefix(l, "'")
			l = strings.TrimPrefix(l, "\"")

			i := strings.IndexFunc(l, func(r rune) bool {
				return r == '\'' || r == '"'
			})
			if i != -1 {
				l = l[:i]
				tsukuruFile.Android.Dependencies = append(tsukuruFile.Android.Dependencies, l)
			}
		}
	}

	return
}

func getDependenciesFromBuildGradle() ([]string, error) {
	buildGradle := filepath.Join(androidDir, "app", "build.gradle")
	f, err := os.Open(buildGradle)
	if err != nil {
		return nil, fmt.Errorf("getDependenciesFromBuildGradle: %w", err)
	}
	defer f.Close()

	dependencies := []string{}

	isDependenciesBlock := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		trimmedLine := strings.TrimSpace(line)

		if !isDependenciesBlock &&
			(strings.HasPrefix(trimmedLine, "dependencies {") ||
				strings.HasPrefix(trimmedLine, "dependencies{")) {
			isDependenciesBlock = true
			continue
		}

		if isDependenciesBlock && strings.HasPrefix(trimmedLine, "}") {
			isDependenciesBlock = false
		}

		if isDependenciesBlock {
			if strings.HasPrefix(trimmedLine, "implementation") {
				l := strings.TrimPrefix(trimmedLine, "implementation")
				l = strings.TrimSpace(l)
				l = strings.TrimPrefix(l, "'")
				l = strings.TrimPrefix(l, "\"")

				i := strings.IndexFunc(l, func(r rune) bool {
					return r == '\'' || r == '"'
				})
				if i != -1 {
					l = l[:i]
					dependencies = append(dependencies, l)
				}
			}
		}
	}

	return dependencies, nil
}

func deduplicate(dependencies []string) ([]string, error) {
	type versionSet map[string]struct{}
	type depsSet map[string]versionSet

	deps := depsSet{}

	for _, v := range dependencies {
		split := strings.Split(v, ":")
		if len(split) > 0 {
			pkg := strings.Join(split[:len(split)-1], ":")
			version := split[len(split)-1]

			_, ok := deps[pkg]
			if !ok {
				deps[pkg] = versionSet{}
			}

			deps[pkg][version] = struct{}{}
		}
	}

	var outDeps []string

	for dep, versions := range deps {
		if len(versions) > 1 {
			// TODO: handle this better
			return nil, errors.New("deduplicate: found multiple versions of '" + dep + "'")
		}

		var version string
		for v := range versions {
			version = v
		}

		outDeps = append(outDeps, dep+":"+version)
	}

	return outDeps, nil
}

func writeDependenciesToBuildGradle(dependencies []string) error {
	buildGradle := filepath.Join(androidDir, "app", "build.gradle")
	f, err := os.Open(buildGradle)
	if err != nil {
		return fmt.Errorf("writeDependenciesToBuildGradle: %w", err)
	}
	defer f.Close()

	var dst bytes.Buffer

	isDependenciesBlock := false

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		trimmedLine := strings.TrimSpace(line)

		if !isDependenciesBlock &&
			(strings.HasPrefix(trimmedLine, "dependencies {") ||
				strings.HasPrefix(trimmedLine, "dependencies{")) {
			isDependenciesBlock = true
		}

		if isDependenciesBlock && strings.HasPrefix(trimmedLine, "}") {
			for _, dep := range dependencies {
				dst.WriteString(fmt.Sprintf("    implementation '%s' // added by tsukuru; DO NOT REMOVE THIS COMMENT\n", dep))
			}
			dst.WriteString(line + "\n")
		} else {
			dst.WriteString(line + "\n")
		}
	}

	err = os.WriteFile(buildGradle, dst.Bytes(), 0666)
	if err != nil {
		return fmt.Errorf("writeDependenciesToBuildGradle: %w", err)
	}

	return nil
}
