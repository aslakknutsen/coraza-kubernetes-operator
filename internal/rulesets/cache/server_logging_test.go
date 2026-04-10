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

package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/networking-incubator/coraza-kubernetes-operator/test/utils"
)

func captureLogger(lines *[]string) logr.Logger {
	return funcr.New(func(prefix, args string) {
		*lines = append(*lines, prefix+args)
	}, funcr.Options{})
}

func TestLoggerFromContext_PrefersContextLogger(t *testing.T) {
	var primaryLines, fallbackLines []string
	primaryLog := captureLogger(&primaryLines)
	fallbackLog := captureLogger(&fallbackLines)
	ctx := logr.NewContext(context.Background(), primaryLog)

	got := loggerFromContext(ctx, fallbackLog)
	got.Info("probe")

	require.NotEmpty(t, primaryLines)
	assert.Empty(t, fallbackLines)
}

func TestLoggerFromContext_FallsBackWhenNoLoggerInContext(t *testing.T) {
	var fallbackLines []string
	fallbackLog := captureLogger(&fallbackLines)

	got := loggerFromContext(context.Background(), fallbackLog)
	got.Info("probe")

	require.NotEmpty(t, fallbackLines)
}

func TestShortID_IsHexSixteenChars(t *testing.T) {
	id := shortID()
	assert.Len(t, id, 16)
	for _, c := range id {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "got %q", id)
	}
}

func TestWithRequestLogger_ForwardsXRequestID(t *testing.T) {
	cache := NewRuleSetCache()
	var lines []string
	server := NewServer(cache, ":0", captureLogger(&lines), nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := loggerFromContext(r.Context(), server.logger)
		log.Info("inner")
		w.WriteHeader(http.StatusOK)
	})

	h := server.withRequestLogger(inner)
	req := httptest.NewRequest(http.MethodGet, "/rules/x", nil)
	req.Header.Set("X-Request-ID", "upstream-req-99")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "upstream-req-99")
	assert.Contains(t, joined, "inner")
}

func TestWithRequestLogger_EmptyXRequestIDGeneratesID(t *testing.T) {
	cache := NewRuleSetCache()
	var lines []string
	server := NewServer(cache, ":0", captureLogger(&lines), nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := loggerFromContext(r.Context(), server.logger)
		log.Info("inner")
		w.WriteHeader(http.StatusOK)
	})

	h := server.withRequestLogger(inner)
	req := httptest.NewRequest(http.MethodGet, "/rules/x", nil)
	req.Header.Set("X-Request-ID", "")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "requestID")
	// Generated id is 16 lowercase hex chars in funcr output
	foundHex := false
	for i := 0; i+15 < len(joined); i++ {
		chunk := joined[i : i+16]
		if isLowerHex(chunk) {
			foundHex = true
			break
		}
	}
	assert.True(t, foundHex, "expected 16-char hex id in log: %q", joined)
}

func isLowerHex(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func TestWithRequestLogger_FromContextLoggerWithoutRequestID(t *testing.T) {
	cache := NewRuleSetCache()
	var lines []string
	base := captureLogger(&lines)
	ctx := logr.NewContext(context.Background(), base.WithValues("suite", "qa"))
	server := NewServer(cache, ":0", utils.NewTestLogger(t), nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := loggerFromContext(r.Context(), server.logger)
		log.Info("inner")
		w.WriteHeader(http.StatusOK)
	})

	h := server.withRequestLogger(inner)
	req := httptest.NewRequest(http.MethodGet, "/rules/x", nil)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "suite")
	assert.Contains(t, joined, "qa")
	assert.Contains(t, joined, "requestID")
}
