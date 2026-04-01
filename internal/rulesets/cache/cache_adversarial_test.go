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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleSetCache_Adversarial_ConcurrentPutGetPrune(t *testing.T) {
	c := NewRuleSetCache()
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers * 3)

	for w := range workers {
		inst := string(rune('A' + (w % 4)))
		go func() {
			defer wg.Done()
			for i := range 50 {
				c.Put(inst, string(rune('a'+i%26)), map[string][]byte{"f": {byte(i)}})
			}
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				_, _ = c.Get(inst)
				_ = c.TotalSize()
				_ = c.ListKeys()
			}
		}()
		go func() {
			defer wg.Done()
			for range 25 {
				c.Prune(time.Hour)
				c.PruneBySize(1 << 20)
			}
		}()
	}
	wg.Wait()
	_, _ = c.Get("A")
}

func TestRuleSetCache_Adversarial_PruneBySize_OnlyLatestPerInstance(t *testing.T) {
	c := NewRuleSetCache()
	// One entry each; each entry is "latest" and cannot be pruned by size.
	c.Put("a", "hello", nil) // 5 bytes
	c.Put("b", "world", nil) // 5 bytes
	require.Equal(t, 10, c.TotalSize())
	n := c.PruneBySize(0)
	assert.Equal(t, 0, n, "cannot prune when every entry is protected as latest")
	assert.Equal(t, 10, c.TotalSize(), "size cannot drop below sum of latest-only entries")
}

func TestRuleSetCache_Adversarial_Put_NilDatafilesNoPanic(t *testing.T) {
	c := NewRuleSetCache()
	require.NotPanics(t, func() {
		c.Put("x", "rules", nil)
	})
	e, ok := c.Get("x")
	require.True(t, ok)
	// Put clones into a new map; nil input becomes an empty (non-nil) map.
	assert.Empty(t, e.DataFiles)
}

func TestRuleSetCache_Adversarial_GetEmptyAndMissing(t *testing.T) {
	c := NewRuleSetCache()
	e, ok := c.Get("anything")
	assert.False(t, ok)
	assert.Nil(t, e)
	c.Put("only", "x", nil)
	e, ok = c.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, e)
}

func TestRuleSetCache_Adversarial_Prune_ZeroMaxAge_KeepsOnlyLatestAndVeryRecent(t *testing.T) {
	c := NewRuleSetCache()
	c.Put("i", "old", nil)
	c.Put("i", "new", nil)
	// Mark non-latest as ancient; latest slightly in past
	c.SetEntryTimestamp("i", 0, time.Now().Add(-time.Hour))
	c.SetEntryTimestamp("i", 1, time.Now().Add(-time.Millisecond))
	p := c.Prune(0)
	assert.Equal(t, 1, p)
	e, ok := c.Get("i")
	require.True(t, ok)
	assert.Equal(t, "new", e.Rules)
}

func TestRuleSetCache_Adversarial_PruneBySize_MaxSizeZero(t *testing.T) {
	c := NewRuleSetCache()
	c.Put("i", "aa", nil)
	c.Put("i", "bb", nil) // latest
	pruned := c.PruneBySize(0)
	assert.GreaterOrEqual(t, pruned, 1)
	assert.LessOrEqual(t, c.TotalSize(), 2+2) // latest "bb" only
}

func TestRuleSetCache_Adversarial_TotalSize_ExactSum(t *testing.T) {
	c := NewRuleSetCache()
	rules := "abcd"
	df := map[string][]byte{"nm": {1, 2, 3}}
	c.Put("i", rules, df)
	want := len(rules) + len("nm") + 3
	assert.Equal(t, want, c.TotalSize())
}

func TestRuleSetCache_Adversarial_MultipleInstancesIsolation(t *testing.T) {
	a := NewRuleSetCache()
	b := NewRuleSetCache()
	a.Put("k", "secret", nil)
	_, ok := b.Get("k")
	assert.False(t, ok)
	e, ok := a.Get("k")
	require.True(t, ok)
	assert.Equal(t, "secret", e.Rules)
}
