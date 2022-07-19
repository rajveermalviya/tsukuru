package androidbuilder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type JavaTools struct {
	Java    string
	Javac   string
	Jar     string
	Keytool string
}

type AndroidBuildTools struct {
	Aapt2     string
	D8        string
	Zipalign  string
	Apksigner string
}

type CustomBuilder struct {
	MinSdkVersion    string
	TargetSdkVersion string

	JavaTools         JavaTools
	AndroidBuildTools AndroidBuildTools
	AndroidJar        string
}

func NewCustomBuilder(androidDir string, autoDownloadPackages bool) (*CustomBuilder, error) {
	minSdk, targetSdk, err := FindMinSdkAndTargetSdk(androidDir)
	if err != nil {
		return nil, err
	}

	javaHome, err := getJavaHome()
	if err != nil {
		return nil, err
	}

	androidSdkRoot, licenses, err := GetAndroidSdkRoot()
	if err != nil {
		return nil, err
	}

	if !licenses {
		sdkmanager := filepath.Join(androidSdkRoot, "platform-tools", "sdkmanager")
		return nil, errors.New("android sdk licenses not accepted, run \"" + sdkmanager + " --licenses\"")
	}

	buildTools, err := findAndroidBuildTools(androidSdkRoot, targetSdk)
	if err != nil {
		if autoDownloadPackages {
			buildTools, err = downloadAndroidBuildtools(androidSdkRoot, targetSdk)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	err = checkAndroidBuildTools(buildTools)
	if err != nil {
		return nil, err
	}

	platformDir, err := findAndroidPlatform(androidSdkRoot, targetSdk)
	if err != nil {
		if autoDownloadPackages {
			platformDir, err = downloadAndroidPlatform(androidSdkRoot, targetSdk)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	err = checkAndroidPlatform(platformDir)
	if err != nil {
		return nil, err
	}

	return &CustomBuilder{
		MinSdkVersion:    minSdk,
		TargetSdkVersion: targetSdk,
		JavaTools: JavaTools{
			Java:    filepath.Join(javaHome, "bin", getName("java")),
			Javac:   filepath.Join(javaHome, "bin", getName("javac")),
			Jar:     filepath.Join(javaHome, "bin", getName("jar")),
			Keytool: filepath.Join(javaHome, "bin", getName("keytool")),
		},
		AndroidBuildTools: AndroidBuildTools{
			Aapt2:     filepath.Join(buildTools, getName("aapt2")),
			D8:        filepath.Join(buildTools, getName("d8")),
			Zipalign:  filepath.Join(buildTools, getName("zipalign")),
			Apksigner: filepath.Join(buildTools, getName("apksigner")),
		},
		AndroidJar: filepath.Join(platformDir, "android.jar"),
	}, nil
}

type customBuildApkOptions struct {
	androidDir string
	targetDir  string

	keystorePath string
	keystorePass string

	javacSourceCompatibility string
	javacTargetCompatibility string
}

type CustomBuildApkOption func(*customBuildApkOptions)

// Use a custom keystore to sign the apk, by default it tries to use
// android debug.keystore located at "$HOME/.android/debug.keystore"
//
// keystorePass arg should be in following forms:
//  pass:<password> password provided inline
//  env:<name>      password provided in the named environment variable
//  file:<file>     password provided in the named file, as a single line
//
// A password is required to open a KeyStore.
func CustomBuildOptKeystore(keystorePath string, keystorePass string) CustomBuildApkOption {
	return func(opts *customBuildApkOptions) {
		opts.keystorePath = keystorePath
		opts.keystorePass = keystorePass
	}
}

func CustomBuildOptJavacCompatibility(source, target string) CustomBuildApkOption {
	return func(opts *customBuildApkOptions) {
		opts.javacSourceCompatibility = source
		opts.javacTargetCompatibility = target
	}
}

func (b *CustomBuilder) BuildApk(androidDir string, targetDir string, opts ...CustomBuildApkOption) (string, error) {
	keystore, err := findOrGenerateDebugKeystore(b.JavaTools.Keytool)
	if err != nil {
		return "", err
	}

	buildOpts := &customBuildApkOptions{
		androidDir: androidDir,
		targetDir:  targetDir,

		javacSourceCompatibility: "8",
		javacTargetCompatibility: "8",

		keystorePath: keystore,
		keystorePass: "pass:android",
	}

	for _, opt := range opts {
		opt(buildOpts)
	}

	return b.buildApk(buildOpts)
}

func (b *CustomBuilder) buildApk(opts *customBuildApkOptions) (string, error) {
	err := os.RemoveAll(opts.targetDir)
	if err != nil {
		return "", err
	}

	err = b.compileResources(opts)
	if err != nil {
		return "", err
	}

	err = b.compileSources(opts)
	if err != nil {
		return "", err
	}

	err = b.mergeApk(opts)
	if err != nil {
		return "", err
	}

	err = b.signApk(opts)
	if err != nil {
		return "", err
	}

	return filepath.Join(opts.targetDir, "app.apk"), nil
}

func (b *CustomBuilder) compileResources(opts *customBuildApkOptions) error {
	appManifest := filepath.Join(opts.androidDir, "app", "src", "main", "AndroidManifest.xml")
	pkg, err := GetPakageFromManifest(appManifest)
	if err != nil {
		return err
	}
	pkgPath := filepath.Join(strings.Split(pkg, ".")...)

	intermediatesDir := filepath.Join(opts.targetDir, "intermediates")

	err = os.MkdirAll(intermediatesDir, 0755)
	if err != nil {
		return fmt.Errorf("compileResources: %w", err)
	}

	resDir := filepath.Join(opts.androidDir, "app", "src", "main", "res")
	resZip := filepath.Join(intermediatesDir, "res.zip")
	err = b.runCmd(exec.Command(b.AndroidBuildTools.Aapt2, "compile", "-o", resZip, "--dir", resDir))
	if err != nil {
		return fmt.Errorf("compileResources: %w", err)
	}

	unalignedApk := filepath.Join(intermediatesDir, "unaligned.apk")

	err = b.runCmd(exec.Command(
		b.AndroidBuildTools.Aapt2, "link",
		"-o", unalignedApk,
		"--manifest", appManifest,
		"-I", b.AndroidJar,
		"--java", intermediatesDir,
		"--output-text-symbols", filepath.Join(intermediatesDir, "R.txt"),
		resZip,
	))
	if err != nil {
		return fmt.Errorf("compileResources: %w", err)
	}

	err = b.runCmd(exec.Command(
		b.JavaTools.Javac,
		"-source", opts.javacSourceCompatibility,
		"-target", opts.javacTargetCompatibility,
		"-bootclasspath", b.AndroidJar,
		"-d", filepath.Join(intermediatesDir, "R"),
		filepath.Join(intermediatesDir, pkgPath, "R.java"),
	))
	if err != nil {
		return fmt.Errorf("compileResources: %w", err)
	}

	err = b.runCmd(exec.Command(
		b.JavaTools.Jar,
		"--create",
		"--file", filepath.Join(intermediatesDir, "R.jar"),
		"-C", filepath.Join(intermediatesDir, "R"),
		".",
	))
	if err != nil {
		return fmt.Errorf("compileResources: %w", err)
	}

	_ = os.Remove(filepath.Join(intermediatesDir, pkgPath, "R.java"))
	return nil
}

func (b *CustomBuilder) compileSources(opts *customBuildApkOptions) error {
	appManifest := filepath.Join(opts.androidDir, "app", "src", "main", "AndroidManifest.xml")

	pkg, err := GetPakageFromManifest(appManifest)
	if err != nil {
		return fmt.Errorf("compileSources: %w", err)
	}
	pkgPath := filepath.Join(strings.Split(pkg, ".")...)

	srcDir := filepath.Join(opts.androidDir, "app", "src")
	var srces []string
	err = fs.WalkDir(os.DirFS(srcDir), ".", func(path string, d fs.DirEntry, _ error) error {
		if d != nil && d.Type().IsRegular() && strings.HasSuffix(path, ".java") {
			srces = append(srces, filepath.Join(srcDir, path))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("compileSources: %w", err)
	}

	intermediatesDir := filepath.Join(opts.targetDir, "intermediates")

	jars := []string{
		b.AndroidJar,
		filepath.Join(intermediatesDir, "R.jar"),
	}

	{
		args := []string{
			"-source", opts.javacSourceCompatibility,
			"-target", opts.javacTargetCompatibility,
			"-classpath", strings.Join(jars, string(os.PathListSeparator)),
			"-d", intermediatesDir,
		}
		args = append(args, srces...)

		err = b.runCmd(exec.Command(b.JavaTools.Javac, args...))
		if err != nil {
			return fmt.Errorf("compileSources: %w", err)
		}
	}

	classesDir := filepath.Join(intermediatesDir, pkgPath)
	var classes []string
	err = fs.WalkDir(os.DirFS(classesDir), ".", func(path string, d fs.DirEntry, _ error) error {
		if d != nil && d.Type().IsRegular() && strings.HasSuffix(path, ".class") {
			classes = append(classes, filepath.Join(classesDir, path))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("compileSources: %w", err)
	}

	{
		args := []string{
			"--classpath", b.AndroidJar,
			"--min-api", b.MinSdkVersion,
			"--output", intermediatesDir,
		}
		args = append(args, classes...)

		err = b.runCmd(exec.Command(b.AndroidBuildTools.D8, args...))
		if err != nil {
			return fmt.Errorf("compileSources: %w", err)
		}
	}

	return nil
}

func (b *CustomBuilder) mergeApk(opts *customBuildApkOptions) error {
	intermediatesDir := filepath.Join(opts.targetDir, "intermediates")
	unaligned := filepath.Join(intermediatesDir, "unaligned.apk")

	// PathOnHost -> PathInZip
	files := map[string]string{
		filepath.Join(intermediatesDir, "classes.dex"): "classes.dex",
	}

	matches, err := filepath.Glob(filepath.Join(opts.androidDir, "app", "src", "main", "jniLibs", "*", "*.so"))
	if err != nil {
		return fmt.Errorf("mergeApk: %w", err)
	}
	for _, match := range matches {
		files[match] = filepath.Join("lib", filepath.Base(filepath.Dir(match)), filepath.Base(match))
	}

	err = addFilesToZip(unaligned, files)
	if err != nil {
		return fmt.Errorf("mergeApk: %w", err)
	}

	err = b.runCmd(exec.Command(
		b.AndroidBuildTools.Zipalign,
		"-f", "4",
		unaligned,
		filepath.Join(intermediatesDir, "aligned.apk"),
	))
	if err != nil {
		return fmt.Errorf("mergeApk: %w", err)
	}

	return nil
}

func (b *CustomBuilder) signApk(opts *customBuildApkOptions) error {
	intermediatesDir := filepath.Join(opts.targetDir, "intermediates")

	err := b.runCmd(exec.Command(
		b.AndroidBuildTools.Apksigner,
		"sign",
		"--ks", opts.keystorePath,
		"--ks-pass", opts.keystorePass,
		"--min-sdk-version", b.MinSdkVersion,
		"--out", filepath.Join(opts.targetDir, "app.apk"),
		filepath.Join(intermediatesDir, "aligned.apk"),
	))
	if err != nil {
		return fmt.Errorf("signApk: %w", err)
	}

	return nil
}

func (b *CustomBuilder) runCmd(cmd *exec.Cmd) error {
	o, err := cmd.CombinedOutput()
	fmt.Println(cmd.String())
	if err != nil {
		os.Stderr.Write(o)
		return err
	}
	return nil
}
