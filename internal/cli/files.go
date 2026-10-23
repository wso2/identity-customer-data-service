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
	"path/filepath"
)

// writeFileAtomic replaces a file in one step.
//
// Enablement edits files another product needs in order to start, so a half
// written deployment.toml is worse than no change at all. Staging beside the
// target keeps the rename on one filesystem, where it is atomic.
func writeFileAtomic(target string, content []byte, mode os.FileMode) error {

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

func copyFile(source, target string, mode os.FileMode) error {

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return writeFileAtomic(target, content, mode)
}

// backupFile keeps one copy of a file before it is first modified, so an
// operator can see what the installation looked like before CDS touched it.
// An existing backup is never overwritten: the first copy is the original.
func backupFile(target string) (string, error) {

	backup := target + ".cds-backup"
	if _, err := os.Stat(backup); err == nil {
		return backup, nil
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", target, err)
	}
	if err := writeFileAtomic(backup, content, 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", backup, err)
	}
	return backup, nil
}
