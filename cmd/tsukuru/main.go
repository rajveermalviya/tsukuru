package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rajveermalviya/tsukuru/androidbuilder"
)

var (
	// provide a different path for android directory,
	// may be useful for other CLIs that build on top of `tsukuru`
	// providing their custom template
	androidDir string

	outputFile     string
	androidBackend string
	libName        string
	ldflags        string
	release        bool
	download       bool
	goarches       string
	x              bool
	a              bool
	race           bool
	tags           string
)

var (
	buildApkCmd       = flag.NewFlagSet("tsukuru build apk", flag.ExitOnError)
	buildAppbundleCmd = flag.NewFlagSet("tsukuru build appbundle", flag.ExitOnError)
	runApkCmd         = flag.NewFlagSet("tsukuru run apk", flag.ExitOnError)
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of tsukuru:\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\ttsukuru build [-options] {apk, appbundle} <path to main package>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\ttsukuru run [-options] apk <path to main package>\n\n")
		flag.PrintDefaults()
	}

	// setup common flags
	for _, c := range []*flag.FlagSet{buildApkCmd, buildAppbundleCmd, runApkCmd} {
		c.StringVar(&androidDir, "androiddir", "", "android directory (default \"android\")")
		c.StringVar(&androidBackend, "androidbackend", "gradle", "builder backend for android, possible values are \"custom\" (experimental), \"gradle\"")
		c.StringVar(&libName, "libname", "main", "name of the shared library, should be exactly same name as passed in System.loadLibrary()")
		c.StringVar(&ldflags, "ldflags", "", "")
		c.BoolVar(&release, "release", false, "currently ignored by \"custom\" backend")
		c.BoolVar(&download, "download", true, "automatically download missing sdks")
		c.StringVar(&goarches, "goarches", "arm64,arm,amd64,386", "comma separated list (no spaces) of GOARCH to include in apk")
		c.BoolVar(&x, "x", false, "")
		c.BoolVar(&a, "a", false, "")
		c.BoolVar(&race, "race", false, "")
		c.StringVar(&tags, "tags", "", "")
	}

	for _, c := range []*flag.FlagSet{buildApkCmd, buildAppbundleCmd} {
		c.StringVar(&outputFile, "o", "", "output path for apk or appbundle")
	}
}

func fail() {
	flag.Usage()
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		fail()
	}

	cmdType := os.Args[1]
	targetType := os.Args[2]

	var mainPackagePath string

	switch {
	case cmdType == "build" && targetType == "apk":
		buildApkCmd.Parse(os.Args[3:])
		mainPackagePath = buildApkCmd.Arg(0)

	case cmdType == "build" && targetType == "appbundle":
		buildAppbundleCmd.Parse(os.Args[3:])
		mainPackagePath = buildAppbundleCmd.Arg(0)

	case cmdType == "run" && targetType == "apk":
		runApkCmd.Parse(os.Args[3:])
		mainPackagePath = runApkCmd.Arg(0)

	default:
		fail()
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	pkg, err := build.Import(mainPackagePath, wd, build.FindOnly)
	if err != nil {
		fmt.Println(err)
		fail()
		return
	}

	mainPackagePath = pkg.Dir

	if androidDir == "" {
		androidDir = filepath.Join(mainPackagePath, "android")
	}

	out := buildAndroid(mainPackagePath, targetType)

	if runApkCmd.Parsed() {
		runAndroid(out)
	}
}

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
		if !Contains(goarchesSlice, goarch) {
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

	if outputFile != "" {
		err := cp(apk, outputFile)
		if err != nil {
			fmt.Printf("failed to copy apk from %s to %s: %v\n", apk, outputFile, err)
			fmt.Printf("but apk is still available at: %s\n", apk)
		} else {
			fmt.Println("Built apk available at:", apk)
		}
	} else {
		fmt.Println("Built apk available at:", apk)
	}

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

	apk, err := b.BuildApk(androidDir, "target")
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

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
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

func cp(src, dst string) error {
	srcf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()

	srcInfo, err := srcf.Stat()
	if err != nil {
		return err
	}

	dstf, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstf.Close()

	_, err = io.Copy(dstf, srcf)
	if err != nil {
		return err
	}

	return nil
}
