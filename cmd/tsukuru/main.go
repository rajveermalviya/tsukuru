package main

import (
	"flag"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
)

var (
	// provide a different path for android directory,
	// may be useful for other CLIs that build on top of `tsukuru`
	// providing their custom template
	androidDir string

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
	skipcheckin    bool

	// for run wasm server
	addr string
)

var (
	buildApkCmd       = flag.NewFlagSet("build apk", flag.ExitOnError)
	buildAppbundleCmd = flag.NewFlagSet("build appbundle", flag.ExitOnError)
	runApkCmd         = flag.NewFlagSet("run apk", flag.ExitOnError)
	buildWasmCmd      = flag.NewFlagSet("build wasm", flag.ExitOnError)
	runWasmCmd        = flag.NewFlagSet("run wasm", flag.ExitOnError)
	checkinCmd        = flag.NewFlagSet("checkin deps", flag.ExitOnError)
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of tsukuru:\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\ttsukuru build {apk, appbundle, wasm} [-options] <path to main package>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\ttsukuru run {apk, wasm} [-options] <path to main package>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\ttsukuru checkin deps [-options] <path to main package>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Run 'tsukuru [command] [subcommand] -help' for details\n\n")
		flag.PrintDefaults()
	}

	// setup common flags
	for _, c := range []*flag.FlagSet{buildApkCmd, buildAppbundleCmd, runApkCmd, buildWasmCmd, runWasmCmd} {
		c.StringVar(&ldflags, "ldflags", "", "")
		c.BoolVar(&release, "release", false, "currently ignored by \"custom\" android backend")
		c.BoolVar(&x, "x", false, "")
		c.BoolVar(&a, "a", false, "")
		c.BoolVar(&race, "race", false, "")
		c.StringVar(&tags, "tags", "", "")
	}

	// setup common android flags
	for _, c := range []*flag.FlagSet{buildApkCmd, buildAppbundleCmd, runApkCmd} {
		c.StringVar(&androidDir, "androiddir", "", "android directory (default \"android\")")
		c.StringVar(&androidBackend, "androidbackend", "gradle", "builder backend for android, possible values are \"custom\" (experimental), \"gradle\"")
		c.StringVar(&libName, "libname", "main", "name of the shared library, should be exactly same name as passed in System.loadLibrary()")
		c.BoolVar(&download, "download", true, "automatically download missing sdks")
		c.StringVar(&goarches, "goarches", "arm64,arm,amd64,386", "comma separated list (no spaces) of GOARCH to include in apk")
		c.BoolVar(&skipcheckin, "skipcheckin", false, "")
	}

	runWasmCmd.StringVar(&addr, "addr", ":8080", "")
}

func fail() {
	flag.Usage()
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		fail()
	}

	mainCmd := os.Args[1]
	subCmd := os.Args[2]

	var mainPackagePath string

	switch {
	case mainCmd == "build" && subCmd == "apk":
		buildApkCmd.Parse(os.Args[3:])
		mainPackagePath = buildApkCmd.Arg(0)

	case mainCmd == "build" && subCmd == "appbundle":
		buildAppbundleCmd.Parse(os.Args[3:])
		mainPackagePath = buildAppbundleCmd.Arg(0)

	case mainCmd == "run" && subCmd == "apk":
		runApkCmd.Parse(os.Args[3:])
		mainPackagePath = runApkCmd.Arg(0)

	case mainCmd == "build" && subCmd == "wasm":
		buildWasmCmd.Parse(os.Args[3:])
		mainPackagePath = buildWasmCmd.Arg(0)

	case mainCmd == "run" && subCmd == "wasm":
		runWasmCmd.Parse(os.Args[3:])
		mainPackagePath = runWasmCmd.Arg(0)

	case mainCmd == "checkin" && subCmd == "deps":
		checkinCmd.Parse(os.Args[3:])
		mainPackagePath = checkinCmd.Arg(0)

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

	switch {
	case buildApkCmd.Parsed():
		if androidDir == "" {
			androidDir = filepath.Join(mainPackagePath, "android")
		}

		if !skipcheckin {
			checkin(mainPackagePath)
		}
		_ = buildAndroid(mainPackagePath, "apk")

	case buildAppbundleCmd.Parsed():
		if androidDir == "" {
			androidDir = filepath.Join(mainPackagePath, "android")
		}

		if !skipcheckin {
			checkin(mainPackagePath)
		}
		_ = buildAndroid(mainPackagePath, "appbundle")

	case runApkCmd.Parsed():
		if androidDir == "" {
			androidDir = filepath.Join(mainPackagePath, "android")
		}

		if !skipcheckin {
			checkin(mainPackagePath)
		}
		out := buildAndroid(mainPackagePath, "apk")
		runAndroid(out)

	case buildWasmCmd.Parsed():
		_ = buildWasm(mainPackagePath, "main.wasm")

	case runWasmCmd.Parsed():
		out := buildWasm(mainPackagePath, "test.wasm")
		runWasm(out)

	case checkinCmd.Parsed():
		checkin(mainPackagePath)
	}
}
