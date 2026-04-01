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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/networking-incubator/coraza-kubernetes-operator/test/utils"
)

func TestServer_Adversarial_DisallowedMethods_POST_PUT_DELETE(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/rules/k", nil)
			w := httptest.NewRecorder()
			srv.handleRules(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestServer_Adversarial_PathLikeKey_NoFilesystemTraversal(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	key := "../../../etc/passwd"
	cache.Put(key, "rules", nil)
	req := httptest.NewRequest(http.MethodGet, "/rules/"+key+"/latest", nil)
	w := httptest.NewRecorder()
	srv.handleRules(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServer_Adversarial_EmptyPathRulesSlash(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	req := httptest.NewRequest(http.MethodGet, "/rules/", nil)
	w := httptest.NewRecorder()
	srv.handleRules(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_Adversarial_VeryLongCacheKey(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	longKey := strings.Repeat("k", 12000)
	cache.Put(longKey, "ok", nil)
	req := httptest.NewRequest(http.MethodGet, "/rules/"+longKey, nil)
	w := httptest.NewRecorder()
	srv.handleRules(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServer_Adversarial_JSONContentTypeOnSuccess(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	cache.Put("x", "{}", nil)
	for _, path := range []string{"/rules/x", "/rules/x/latest"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.handleRules(w, req)
		assert.Equal(t, http.StatusOK, w.Code, path)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), path)
	}
}

func TestServer_Adversarial_ConcurrentGET(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	cache.Put("c", "data", nil)
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/rules/c", nil)
			w := httptest.NewRecorder()
			srv.handleRules(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}
	wg.Wait()
}

func TestServer_Adversarial_GETWithBody_NotRejectedByServerConfig(t *testing.T) {
	cache := NewRuleSetCache()
	logger := utils.NewTestLogger(t)
	srv := NewServer(cache, ":0", logger, nil)
	cache.Put("b", "x", nil)
	body := strings.NewReader(strings.Repeat("z", 4096))
	req := httptest.NewRequest(http.MethodGet, "/rules/b", body)
	req.ContentLength = 4096
	w := httptest.NewRecorder()
	srv.handleRules(w, req)
	// MaxBodySize is documented as 0 but not applied to http.Server — handler still succeeds.
	assert.Equal(t, http.StatusOK, w.Code)
}
