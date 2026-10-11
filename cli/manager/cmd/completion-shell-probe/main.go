// This temporary diagnostic compares shell process paths without retrying a
// failed assertion or changing the completion adapter under test.
package main

import (
	"bytes"
	"context"
	"debug/pe"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wago-org/wago/cli/manager/internal/config"
)

func main() {
	helper := flag.String("helper", "", "native config test executable")
	count := flag.Int("count", 10, "independent observations per shell and stage")
	flag.Parse()
	if *helper == "" || *count < 1 {
		panic("-helper and a positive -count are required")
	}
	root, err := os.MkdirTemp("", "wago-completion-probe-")
	check(err)
	defer os.RemoveAll(root)
	name := "wago"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(*helper)
	check(err)
	check(os.WriteFile(filepath.Join(root, name), data, 0o755))
	script, err := config.Completion("bash")
	check(err)
	path := filepath.Join(root, "wago.bash")
	check(os.WriteFile(path, []byte(script), 0o644))
	bash, err := exec.LookPath("bash")
	check(err)
	shells := []string{bash}
	if runtime.GOOS == "windows" {
		shells = append(shells, filepath.Join(filepath.Dir(bash), "..", "usr", "bin", "bash.exe"), `C:\msys64\usr\bin\bash.exe`)
	}
	fmt.Printf("parent=%s/%s Go=%s helper=%s\n", runtime.GOOS, runtime.GOARCH, runtime.Version(), *helper)
	identity(*helper)
	env := append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"), "WAGO_COMPLETION_TEST_HELPER=1")
	stages := []struct{ name, script string }{
		{"startup", `printf 'install\n'`},
		{"native-helper", `command wago __complete version ""`},
		{"generated-completion", `. "$1"; COMP_WORDS=(wago version ""); COMP_CWORD=2; _wago_complete; printf '%s\n' "${COMPREPLY[@]}"`},
	}
	failed := 0
	for _, shell := range shells {
		identity(shell)
		if _, err := os.Stat(shell); err != nil {
			fmt.Printf("FAIL shell=%q missing: %v\n", shell, err)
			failed++
			continue
		}
		stdout, stderr, err := observe(shell, `printf 'BASH_VERSION=%s\nMACHTYPE=%s\nMSYSTEM=%s\nPATH=%s\n' "$BASH_VERSION" "$MACHTYPE" "$MSYSTEM" "$PATH"; command -v wago`, path, env)
		fmt.Printf("environment shell=%q err=%v stdout=%q stderr=%q\n", shell, err, stdout, stderr)
		if err != nil {
			failed++
		}
		for _, stage := range stages {
			passed := 0
			for n := 1; n <= *count; n++ {
				stdout, stderr, err := observe(shell, stage.script, path, env)
				if err != nil || strings.TrimSpace(stdout) != "install" {
					failed++
					fmt.Printf("FAIL shell=%q stage=%s observation=%d err=%v stdout=%q stderr=%q\n", shell, stage.name, n, err, stdout, stderr)
				} else {
					passed++
				}
			}
			fmt.Printf("SUMMARY shell=%q stage=%s passed=%d failed=%d\n", shell, stage.name, passed, *count-passed)
		}
	}
	if failed != 0 {
		fmt.Printf("TOTAL failed observations=%d; every observed failure remains a failure\n", failed)
		os.Exit(1)
	}
}

func observe(shell, script, path string, env []string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "--noprofile", "--norc", "-x", "-c", script, "_", path)
	cmd.Env = env
	cmd.WaitDelay = 3 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func identity(path string) {
	f, err := pe.Open(path)
	if err != nil {
		fmt.Printf("identity path=%q PE=%v\n", path, err)
		return
	}
	defer f.Close()
	dlls, err := f.ImportedLibraries()
	fmt.Printf("identity path=%q PE_machine=0x%04x DLLs=%v err=%v\n", path, f.Machine, dlls, err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
