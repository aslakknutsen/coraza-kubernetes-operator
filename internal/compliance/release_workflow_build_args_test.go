/*
Copyright Coraza Kubernetes Operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package compliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Tagged release images must receive VERSION and GIT_COMMIT so the manager binary
// logs correct service identity (see Dockerfile ldflags and cmd/manager/main.go).
func TestReleaseWorkflow_ManagerImageBuildArgs(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	path := filepath.Join(repoRoot, ".github", "workflows", "release.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(b)

	// docker/build-push-action step must pass build-args consumed by Dockerfile ARG VERSION / GIT_COMMIT.
	if !strings.Contains(s, "VERSION=${{ github.ref_name }}") {
		t.Error("release workflow must pass VERSION=github.ref_name to the manager image build")
	}
	if !strings.Contains(s, "GIT_COMMIT=${{ github.sha }}") {
		t.Error("release workflow must pass GIT_COMMIT=github.sha to the manager image build")
	}
	if !strings.Contains(s, "docker/build-push-action@") {
		t.Error("expected docker/build-push-action in release workflow for image publish")
	}
}
