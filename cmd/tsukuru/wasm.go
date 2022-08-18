package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildWasm(mainPackagePath string, out string) string {
	wasmPath := filepath.Join("target", "wasm", out)
	_ = os.RemoveAll(filepath.Dir(wasmPath))

	if release {
		ldflags += " -s -w"
	}

	args := []string{
		"build",
		"-trimpath",
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
		"-o", wasmPath,
		mainPackagePath,
	)

	cmd := exec.Command("go", args...)
	cmd.Env = append(
		os.Environ(),
		"GOOS=js",
		"GOARCH=wasm",
	)
	fmt.Println(cmd.String())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	fmt.Println("Built wasm available at:", wasmPath)
	return wasmPath
}

func runWasm(wasm string) {
	var goroot string
	{
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			panic(err)
		}
		goroot = strings.TrimSpace(string(out))
	}

	outDir := filepath.Dir(wasm)

	wasmExecJs := filepath.Join(goroot, "misc", "wasm", "wasm_exec.js")
	fmt.Println("cp", wasmExecJs, filepath.Join(outDir, "wasm_exec.js"))
	err := cp(wasmExecJs, filepath.Join(outDir, "wasm_exec.js"))
	if err != nil {
		panic(err)
	}

	wasmExecHtml := filepath.Join(goroot, "misc", "wasm", "wasm_exec.html")
	fmt.Println("cp", wasmExecHtml, filepath.Join(outDir, "index.html"))
	err = cp(wasmExecHtml, filepath.Join(outDir, "index.html"))
	if err != nil {
		panic(err)
	}

	fmt.Printf("serving %s at %s\n", filepath.Dir(wasm), addr)
	err = http.ListenAndServe(addr, http.FileServer(http.FS(os.DirFS(filepath.Dir(wasm)))))
	if err != nil {
		panic(err)
	}
}
