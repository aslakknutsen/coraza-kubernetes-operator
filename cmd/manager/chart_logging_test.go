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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from cwd")
		}
		dir = parent
	}
}

func TestValuesYAML_DefaultLoggingNotDevelopment(t *testing.T) {
	root := testRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "charts/coraza-kubernetes-operator/values.yaml"))
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "logging:")
	assert.Contains(t, s, "development: false")
}

func TestChartREADME_ListsLoggingDevelopment(t *testing.T) {
	root := testRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "charts/coraza-kubernetes-operator/README.md"))
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "`logging.development`")
}

func TestDeploymentTemplate_PassesZapDevelFromValues(t *testing.T) {
	root := testRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "charts/coraza-kubernetes-operator/templates/deployment.yaml"))
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "--zap-devel={{ .Values.logging.development }}")
}

func TestMakefile_DeploySetsLoggingDevelopmentTrue(t *testing.T) {
	root := testRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "Makefile"))
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "--set logging.development=true")
}

func TestManagerHelp_IncludesZapFlags(t *testing.T) {
	root := testRepoRoot(t)
	cmd := exec.Command("go", "run", "-tags", "no_fs_access", "./cmd/manager", "--help")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	s := string(out)
	assert.Contains(t, s, "-zap-devel")
	assert.Contains(t, s, "-zap-encoder")
}

func TestHelmTemplate_ZapDevelMatchesChartDefaults(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm not on PATH; install helm to run this check locally")
	}
	root := testRepoRoot(t)
	chart := filepath.Join(root, "charts/coraza-kubernetes-operator")
	cmd := exec.Command(helm, "template", "rel", chart, "--namespace", "coraza-system")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "--zap-devel=false")

	cmd = exec.Command(helm, "template", "rel", chart, "--namespace", "coraza-system", "--set", "logging.development=true")
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "--zap-devel=true")
}

func TestReleaseManifestHelmInvocation_DoesNotForceDevLogging(t *testing.T) {
	// build.installer and release.manifests must not set logging.development;
	// default chart values keep production JSON logging.
	root := testRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "Makefile"))
	require.NoError(t, err)
	s := string(b)

	idx := strings.Index(s, "release.manifests:")
	require.Greater(t, idx, -1)
	rest := s[idx:]
	end := strings.Index(rest, "\n\n.PHONY:")
	if end == -1 {
		end = len(rest)
	}
	block := rest[:end]
	assert.NotContains(t, block, "logging.development")

	idx = strings.Index(s, "build.installer:")
	require.Greater(t, idx, -1)
	rest = s[idx:]
	end = strings.Index(rest, "\n\n.PHONY:")
	if end == -1 {
		end = len(rest)
	}
	block = rest[:end]
	assert.NotContains(t, block, "logging.development")
}
