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

// Package compliance holds static checks for release / audit surfaces (Helm, etc.).
package compliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHelmDeploymentTemplate_LoggingComplianceFields(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/compliance -> repo root is ../..
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	deploymentPath := filepath.Join(repoRoot, "charts", "coraza-kubernetes-operator", "templates", "deployment.yaml")
	b, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("read %s: %v", deploymentPath, err)
	}
	s := string(b)

	const wantZapTime = `--zap-time-encoding={{ .Values.logging.timeEncoding | default "rfc3339nano" }}`
	if !strings.Contains(s, wantZapTime) {
		t.Errorf("deployment template must contain %q for audit-visible timestamps", wantZapTime)
	}

	if !strings.Contains(s, "logging.includePodMetadata") {
		t.Error("deployment template must gate POD_NAME/NODE_NAME on logging.includePodMetadata")
	}
	if !strings.Contains(s, "POD_NAME") || !strings.Contains(s, "NODE_NAME") {
		t.Error("deployment template must optionally expose pod/node via Downward API env")
	}

	valuesPath := filepath.Join(repoRoot, "charts", "coraza-kubernetes-operator", "values.yaml")
	vb, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("read %s: %v", valuesPath, err)
	}
	vs := string(vb)
	if !strings.Contains(vs, "timeEncoding:") || !strings.Contains(vs, "includePodMetadata:") {
		t.Error("values.yaml should document logging.timeEncoding and logging.includePodMetadata defaults")
	}
}

func TestDockerfile_ManagerInjectedBuildMetadata(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	dockerfilePath := filepath.Join(repoRoot, "Dockerfile")
	b, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read %s: %v", dockerfilePath, err)
	}
	s := string(b)
	if !strings.Contains(s, "-X main.version=${VERSION}") || !strings.Contains(s, "-X main.gitCommit=${GIT_COMMIT}") {
		t.Error("manager image Dockerfile must link main.version and main.gitCommit for process-level service identity")
	}
}
