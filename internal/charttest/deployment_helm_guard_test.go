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

package charttest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Helm's Go-template truthiness treats numeric 0 as false, so
// `if .Values.istio.prerequisitesReconcileInterval` dropped the CLI flag for
// users who set 0 to disable periodic reconcile. Guard against regression.
func TestHelmDeployment_IstioPrerequisitesIntervalUsesNonEmptyStringGuard(t *testing.T) {
	t.Parallel()
	path := deploymentYAMLPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const want = `ne (toString .Values.istio.prerequisitesReconcileInterval) ""`
	if !strings.Contains(string(data), want) {
		t.Fatalf("deployment template must contain %q so 0 / 0s values still render the flag", want)
	}
}

func deploymentYAMLPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/charttest -> repo root -> charts/.../deployment.yaml
	return filepath.Join(filepath.Dir(file), "..", "..", "charts", "coraza-kubernetes-operator", "templates", "deployment.yaml")
}
