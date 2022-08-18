package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rajveermalviya/tsukuru/androidbuilder"
)

func buildAndroid(mainPackagePath string, targetType string) string {
	minSdk, _, err := androidbuilder.FindMinSdkAndTargetSdk(androidDir)
	if err != nil {
		panic(err)
	}

	androidSdkRoot, _, err := androidbuilder.GetAndroidSdkRoot()
	if err != nil {
		panic(err)
	}

	if !androidbuilder.HasNdk(androidSdkRoot) && download {
		latestVersion, err := androidbuilder.FindLatestVersionOfSdk(
			"ndk",
			"", /* ignored for ndk */
			true,
		)
		if err != nil {
			panic(err)
		}

		err = androidbuilder.DownloadNdk(androidSdkRoot, latestVersion)
		if err != nil {
			panic(err)
		}
	}

	ndkDir := androidbuilder.FindLatestVersionOfNdkInstalled(androidSdkRoot)
	if ndkDir == "" {
		panic("unable to find ndk dir in " + androidSdkRoot)
	}

	type abiForCompiler struct {
		abi    string
		target string
	}

	goarchesSlice := strings.Split(goarches, ",")

	abis := map[string]abiForCompiler{
		"arm": {
			abi:    "armeabi-v7a",
			target: "armv7-none-linux-androideabi",
		},
		"arm64": {
			abi:    "arm64-v8a",
			target: "aarch64-none-linux-android",
		},
		"386": {
			abi:    "x86",
			target: "i686-none-linux-android",
		},
		"amd64": {
			abi:    "x86_64",
			target: "x86_64-none-linux-android",
		},
	}

	if release {
		ldflags += " -s -w"
	}

	toolchainOS := ""
	switch runtime.GOOS {
	case "windows":
		toolchainOS = "windows-x86_64"
	case "darwin":
		toolchainOS = "darwin-x86_64"
	case "linux":
		toolchainOS = "linux-x86_64"
	default:
		panic("invalid GOOS")
	}

	for goarch, abi := range abis {
		libPath := filepath.Join(androidDir, "app", "src", "main", "jniLibs", abi.abi, "lib"+libName+".so")
		_ = os.Remove(libPath)

		// skip GOARCH values that are not in user allowed list
		if !contains(goarchesSlice, goarch) {
			continue
		}

		cc := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", toolchainOS, "bin", "clang")
		cxx := cc + "++"

		gccToolchain := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", toolchainOS)
		sysroot := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", toolchainOS, "sysroot")

		ccEnv := cc + " --target=" + abi.target + minSdk + " --gcc-toolchain=" + gccToolchain + " --sysroot=" + sysroot
		cxxEnv := cxx + " --target=" + abi.target + minSdk + " --gcc-toolchain=" + gccToolchain + " --sysroot=" + sysroot

		args := []string{
			"build",
			"-trimpath",
			"-buildmode", "c-shared",
		}
		if ldflags != "" {
			args = append(args, "-ldflags", ldflags)
		}
		if x {
			args = append(args, "-x")
		}
		if a {
			args = append(args, "-a")
		}
		if race {
			args = append(args, "-race")
		}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args,
			"-o", libPath,
			mainPackagePath,
		)

		cmd := exec.Command("go", args...)
		cmd.Env = append(
			os.Environ(),
			"CGO_ENABLED=1",
			"GOOS=android",
			"GOARCH="+goarch,
			"CC="+ccEnv,
			"CXX="+cxxEnv,
		)
		if goarch == "arm" {
			cmd.Env = append(cmd.Env, "GOARM=7")
		}
		fmt.Println(cmd.String())
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			panic(err)
		}

		_ = os.Remove(filepath.Join(androidDir, "app", "src", "main", "jniLibs", abi.abi, "lib"+libName+".h"))
	}

	var apk string
	switch androidBackend {
	case "gradle":
		apk = gradleBuildAndroid(targetType)
	case "custom":
		apk = customBuildAndroid(targetType)
	default:
		panic("invalid backend")
	}

	fmt.Println("Built apk available at:", apk)
	return apk
}

func customBuildAndroid(targetType string) string {
	if targetType == "appbundle" {
		panic("custom backend doesn't support building appbundle")
	}

	b, err := androidbuilder.NewCustomBuilder(androidDir, download)
	if err != nil {
		panic(err)
	}

	apk, err := b.BuildApk(androidDir, filepath.Join("target", "android"))
	if err != nil {
		panic(err)
	}

	return apk
}

func gradleBuildAndroid(targetType string) string {
	b, err := androidbuilder.NewGradleBuilder()
	if err != nil {
		panic(err)
	}

	var opts []androidbuilder.GradleBuildApkOption
	if release {
		opts = append(opts, androidbuilder.GradleBuilderOptRelease())
	}

	switch targetType {
	case "apk":
		apk, err := b.BuildApk(androidDir, opts...)
		if err != nil {
			panic(err)
		}

		return apk

	case "appbundle":
		aab, err := b.BuildAppbundle(androidDir, opts...)
		if err != nil {
			panic(err)
		}

		return aab

	default:
		panic("invalid target type")
	}
}

func runAndroid(apk string) {
	androidSdkRoot, _, err := androidbuilder.GetAndroidSdkRoot()
	if err != nil {
		panic(err)
	}

	adb := filepath.Join(androidSdkRoot, "platform-tools", "adb")
	if runtime.GOOS == "windows" {
		adb += ".exe"
	}

	{
		cmd := exec.Command(adb, "install", apk)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		fmt.Println(cmd.String())
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	pkgName, activityName, err := findPackageAndActivity()
	if err != nil {
		panic(err)
	}

	{
		cmd := exec.Command(adb, "shell", "am", "start", "-W", "-n", pkgName+"/"+pkgName+activityName)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		fmt.Println(cmd.String())
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	var pid string
	{
		cmd := exec.Command(adb, "shell", "pidof", pkgName)
		fmt.Println(cmd.String())
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}

		pids := strings.Split(strings.TrimSpace(string(out)), " ")
		if len(pids) > 0 {
			pid = pids[0]
		}
	}

	if pid == "" {
		panic("failed to get pid")
	}

	{
		cmd := exec.Command(adb, "logcat", "--pid", pid)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		fmt.Println(cmd.String())
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}
}

func findPackageAndActivity() (pkgName string, activityName string, err error) {
	manifestFile := filepath.Join(androidDir, "app", "src", "main", "AndroidManifest.xml")

	var manifest struct {
		XMLName     xml.Name `xml:"manifest"`
		Package     string   `xml:"package,attr"`
		Application struct {
			Activity []struct {
				Name         string `xml:"name,attr"`
				IntentFilter []struct {
					Action struct {
						Name string `xml:"name,attr"`
					} `xml:"action"`
				} `xml:"intent-filter"`
			} `xml:"activity"`
		} `xml:"application"`
	}

	f, err := os.Open(manifestFile)
	if err != nil {
		return "", "", fmt.Errorf("findPackageNameAndActivityName: %w", err)
	}
	defer f.Close()

	d := xml.NewDecoder(f)
	err = d.Decode(&manifest)
	if err != nil {
		return "", "", fmt.Errorf("findPackageNameAndActivityName: %w", err)
	}

	for _, activity := range manifest.Application.Activity {
		for _, intentFilter := range activity.IntentFilter {
			if intentFilter.Action.Name == "android.intent.action.MAIN" {
				return manifest.Package, activity.Name, nil
			}
		}
	}

	return "", "", errors.New("unable to find")
}
