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

package memfs_test

import (
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/networking-incubator/coraza-kubernetes-operator/internal/rulesets/memfs"
)

func TestMemFS_Adversarial_OpenNonExistent_ErrNotExist(t *testing.T) {
	m := memfs.NewMemFS()
	_, err := m.Open("nope")
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestMemFS_Adversarial_WriteThenReadIntegrity(t *testing.T) {
	m := memfs.NewMemFS()
	want := []byte{0, 1, 2, 3, 255}
	m.WriteFile("bin", want)
	f, err := m.Open("bin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	got, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemFS_Adversarial_ConcurrentReadWrite(t *testing.T) {
	m := memfs.NewMemFS()
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			m.WriteFile("w", []byte{byte(i)})
		}(i)
		go func() {
			defer wg.Done()
			f, err := m.Open("w")
			if err != nil {
				return
			}
			_, _ = io.ReadAll(f)
			_ = f.Close()
		}()
	}
	wg.Wait()
}

func TestMemFS_Adversarial_EmptyFilename(t *testing.T) {
	m := memfs.NewMemFS()
	m.WriteFile("", []byte("x"))
	f, err := m.Open("")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, []byte("x"), b)
}

func TestMemFS_Adversarial_NilData(t *testing.T) {
	m := memfs.NewMemFS()
	require.NotPanics(t, func() { m.WriteFile("nilf", nil) })
	f, err := m.Open("nilf")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Empty(t, b)
}

func TestMemFS_Adversarial_ModTimeChangesOnEachStat(t *testing.T) {
	m := memfs.NewMemFS()
	m.WriteFile("t", []byte("a"))
	f, err := m.Open("t")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	s1, err := f.Stat()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	s2, err := f.Stat()
	require.NoError(t, err)
	assert.NotEqual(t, s1.ModTime(), s2.ModTime(),
		"memFile.ModTime returns time.Now() on each Stat(), so metadata is unstable across calls")
}

func TestMemFS_Adversarial_NotReadDirFS_WalkDirFails(t *testing.T) {
	m := memfs.NewMemFS()
	m.WriteFile("a.conf", []byte("x"))
	_, ok := any(m).(fs.ReadDirFS)
	assert.False(t, ok, "MemFS does not implement fs.ReadDirFS")
	err := fs.WalkDir(m, ".", func(path string, d fs.DirEntry, err error) error {
		return err
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
