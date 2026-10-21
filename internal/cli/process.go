/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// readPID returns the pid of the running CDS process, or 0 when nothing is
// running. A pid file left behind by a crashed process names a pid that is
// gone, or one that now belongs to something else; both read as "not running"
// so that a stale file never blocks a start.
func readPID(cdsHome string) int {

	raw, err := os.ReadFile(path(cdsHome, pidFile))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0
	}
	if !alive(pid) {
		return 0
	}
	return pid
}

// alive reports whether a process with this pid exists and can be signalled.
func alive(pid int) bool {

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 performs the permission and existence checks without
	// delivering anything.
	return proc.Signal(syscall.Signal(0)) == nil
}

func writePID(cdsHome string, pid int) error {

	if err := os.WriteFile(path(cdsHome, pidFile), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", pidFile, err)
	}
	return nil
}

// runStart starts the CDS runtime.
func runStart(args []string) error {

	flags := newFlagSet("start")
	debug := flags.Bool("debug", false, "run in the foreground and log to the terminal")
	timeout := flags.Duration("timeout", 2*time.Minute, "how long to wait for CDS to become ready")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}

	// An unconfigured installation is not started and nothing is acquired on
	// its behalf; the operator is told what is missing instead.
	if _, err := loadIntegration(cdsHome); err != nil {
		return err
	}

	if pid := readPID(cdsHome); pid != 0 {
		info("CDS is already running (pid %d)", pid)
		return nil
	}

	if *debug {
		step("Starting CDS in the foreground")
		info("CDS_HOME %s", cdsHome)
		if err := applySecrets(cdsHome); err != nil {
			return err
		}
		runtimeRun(cdsHome)
		return nil
	}

	step("Starting CDS")
	cfg, err := readRuntimeConfig(cdsHome)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the wso2cds executable: %w", err)
	}

	if err := os.MkdirAll(path(cdsHome, logsDir), 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", logsDir, err)
	}
	logFile, err := os.OpenFile(path(cdsHome, runtimeLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", runtimeLog, err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(exe, "run")
	cmd.Dir = cdsHome
	cmd.Env = append(os.Environ(), "CDS_HOME="+cdsHome)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the CDS runtime: %w", err)
	}

	// Read the pid before anything releases the handle: Release sets it to
	// -1, and a pid file saying -1 is one stop and status can never act on.
	pid := cmd.Process.Pid
	if err := writePID(cdsHome, pid); err != nil {
		return err
	}
	info("pid %d, logging to %s", pid, runtimeLog)

	// Wait on the child while it starts, so an exit is observed rather than
	// inferred. Signalling the pid is not enough: an exited child that has
	// not been reaped is a zombie, and signal 0 to a zombie succeeds — so a
	// runtime that died would look alive for as long as this process ran.
	//
	// Once start returns, the goroutine is abandoned and the child is
	// reparented, which is what detaching means.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitReady(cfg.localBase(), exited, *timeout); err != nil {
		return fmt.Errorf("%w\n\n%s", err, logTail(cdsHome, 15))
	}
	done("CDS is ready at %s", cfg.localBase())
	return nil
}

// logTail quotes the end of the runtime log, so a failed start explains itself
// where the operator is looking rather than asking them to go and find it.
func logTail(cdsHome string, lines int) string {

	file, err := os.Open(path(cdsHome, runtimeLog))
	if err != nil {
		return "See " + runtimeLog + "."
	}
	defer func() { _ = file.Close() }()

	tail, _, err := tailLines(file, lines)
	if err != nil || strings.TrimSpace(tail) == "" {
		return "See " + runtimeLog + "."
	}
	return "The last lines of " + runtimeLog + ":\n\n" + tail
}

// runStop stops the CDS runtime, giving it the chance to shut down on its own
// before it is killed.
func runStop(args []string) error {

	flags := newFlagSet("stop")
	grace := flags.Duration("grace", 60*time.Second, "how long to wait for a graceful shutdown")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}

	pid := readPID(cdsHome)
	if pid == 0 {
		info("CDS is not running")
		_ = os.Remove(path(cdsHome, pidFile))
		return nil
	}

	step("Stopping CDS (pid %d)", pid)
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not signal pid %d: %w", pid, err)
	}

	// SIGTERM is what the runtime installs a handler for: it stops taking
	// requests, drains the workers and closes the database pool.
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("could not stop pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(*grace)
	for time.Now().Before(deadline) {
		if !alive(pid) {
			_ = os.Remove(path(cdsHome, pidFile))
			done("CDS stopped")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	warn("CDS did not stop within %s, killing pid %d", *grace, pid)
	if err := proc.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("could not kill pid %d: %w", pid, err)
	}
	_ = os.Remove(path(cdsHome, pidFile))
	done("CDS killed")
	return nil
}

func runRestart(args []string) error {

	if err := runStop(nil); err != nil {
		return err
	}
	return runStart(args)
}

// runForeground is the hidden `run` command: the runtime itself, in the
// foreground, with no lifecycle handling around it.
func runForeground(args []string) error {

	flags := newFlagSet("run")
	if err := parse(flags, args); err != nil {
		return err
	}
	cdsHome, err := home()
	if err != nil {
		return err
	}
	if err := applySecrets(cdsHome); err != nil {
		return err
	}
	runtimeRun(cdsHome)
	return nil
}

// runtimeDefaults are values the packed configuration expands from the
// environment but that an installed pack has no other source for.
//
// Only what stops the runtime starting is defaulted here: an unset log level
// is not an empty log level, it is a parse failure that ends the process
// before it writes a line saying so. Anything an operator is expected to
// decide — the datasource, the certificate — is left to them and reported
// when it is missing.
var runtimeDefaults = map[string]string{
	"LOG_LEVEL": "INFO",
}

// applySecrets puts the stored credentials into the environment, which is
// where the ${...} placeholders in deployment.yaml are expanded from. A
// variable already set in the environment wins, so a deployment supplying its
// secrets from elsewhere is not overridden by the file.
func applySecrets(cdsHome string) error {

	secrets, err := loadSecrets(cdsHome)
	if err != nil {
		return err
	}
	for _, entry := range environment(secrets) {
		key, value, _ := strings.Cut(entry, "=")
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("could not set %s: %w", key, err)
		}
	}

	for key, value := range runtimeDefaults {
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("could not set %s: %w", key, err)
		}
	}
	return nil
}
