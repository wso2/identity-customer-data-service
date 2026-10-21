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
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// runLogs displays, or follows, the log the detached runtime writes.
//
// A foreground run (`start --debug`) logs to the terminal and writes nothing
// here, so an empty file after a debug session is expected rather than a loss.
func runLogs(args []string) error {

	flags := newFlagSet("logs")
	follow := flags.Bool("f", false, "follow the log as it is written")
	lines := flags.Int("n", 50, "number of trailing lines to show first")
	if err := parse(flags, args); err != nil {
		return err
	}

	cdsHome, err := home()
	if err != nil {
		return err
	}

	target := path(cdsHome, runtimeLog)
	file, err := os.Open(target)
	if err != nil {
		return fmt.Errorf("could not open %s: %w\nCDS writes this file only when started "+
			"without --debug", runtimeLog, err)
	}
	defer func() { _ = file.Close() }()

	offset, err := printTail(file, *lines)
	if err != nil {
		return err
	}
	if !*follow {
		return nil
	}

	// Ctrl-C ends the follow without the caller seeing a signal-killed exit.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	for {
		select {
		case <-interrupt:
			fmt.Println()
			return nil
		default:
		}

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("could not stat %s: %w", runtimeLog, err)
		}
		// A shrunken file means the log was rotated or truncated under us;
		// following from the old offset would print nothing forever.
		if info.Size() < offset {
			offset = 0
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("could not rewind %s: %w", runtimeLog, err)
			}
		}
		if info.Size() > offset {
			written, err := io.Copy(os.Stdout, file)
			if err != nil {
				return fmt.Errorf("could not read %s: %w", runtimeLog, err)
			}
			offset += written
			continue
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// printTail writes the last n lines of the file and returns the offset it read
// up to, so a follow continues from exactly there.
func printTail(file *os.File, n int) (int64, error) {

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("could not stat the log: %w", err)
	}
	size := info.Size()

	// Read backwards in blocks until n newlines have been seen, rather than
	// holding a log of unknown size in memory.
	const block = 16 << 10
	var (
		found int
		start = size
		buf   = make([]byte, block)
	)
	for start > 0 && found <= n {
		readAt := start - block
		if readAt < 0 {
			readAt = 0
		}
		count, err := file.ReadAt(buf[:start-readAt], readAt)
		if err != nil && err != io.EOF {
			return 0, fmt.Errorf("could not read the log: %w", err)
		}
		chunk := buf[:count]
		for i := len(chunk) - 1; i >= 0; i-- {
			if chunk[i] != '\n' {
				continue
			}
			found++
			if found > n {
				start = readAt + int64(i) + 1
				break
			}
		}
		if found > n {
			break
		}
		start = readAt
	}

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return 0, fmt.Errorf("could not seek the log: %w", err)
	}
	tail, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("could not read the log: %w", err)
	}
	fmt.Print(string(tail))
	if len(tail) > 0 && !strings.HasSuffix(string(tail), "\n") {
		fmt.Println()
	}
	return size, nil
}
