package corerulesetgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitIntoRules_multilineTrailingSpaceAfterBackslash(t *testing.T) {
	// Backslash continuation with trailing spaces before newline (common in CRS).
	line1 := `SecRule ARGS "@rx x" "id:1,pass" \   `
	content := line1 + "\n" + `"cont" "id:2,pass"`
	blocks := splitIntoRules(content)
	require.Len(t, blocks, 1)
	require.Contains(t, blocks[0], "id:1")
	require.Contains(t, blocks[0], "cont")
}

func TestChainSecRuleGroups(t *testing.T) {
	t.Run("standalone", func(t *testing.T) {
		blocks := splitIntoRules(`SecRule A "op" "id:1,pass"
SecRule B "op" "id:2,pass"`)
		g := chainSecRuleGroups(blocks)
		require.Len(t, g, 2)
		require.Equal(t, []int{0}, g[0])
		require.Equal(t, []int{1}, g[1])
	})

	t.Run("two_rule_chain", func(t *testing.T) {
		blocks := splitIntoRules(`SecRule ARGS "@rx x" "id:1,phase:2,pass,chain"
SecRule ARGS "@pmFromFile f" "id:2,phase:2,pass"`)
		g := chainSecRuleGroups(blocks)
		require.Len(t, g, 1)
		require.Equal(t, []int{0, 1}, g[0])
	})

	t.Run("comment_between_chained_rules", func(t *testing.T) {
		blocks := splitIntoRules(`SecRule ARGS "@rx x" "id:1,chain"
# comment
SecRule ARGS "@rx y" "id:2,pass"`)
		g := chainSecRuleGroups(blocks)
		require.Len(t, g, 1)
		require.Equal(t, []int{0, 2}, g[0])
	})
}

func TestProcessFileContent_dropsFullChainWhenPMIgnored(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.conf")
	content := `SecRule ARGS "@rx x" "id:1,phase:2,pass,nolog,chain"
SecRule ARGS "@pmFromFile foo.data" "id:2,phase:2,pass,nolog"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	out, warns, err := processFileContent(path, nil, true)
	require.NoError(t, err)
	require.NotContains(t, out, "chain")
	require.NotContains(t, out, "id:1")
	require.NotContains(t, out, "id:2")
	require.True(t, strings.Contains(strings.Join(warns, ""), "SecRule chain"))
}

func TestProcessFileContent_dropsFullChainWhenIDIgnored(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.conf")
	content := `SecRule ARGS "@rx x" "id:10,phase:2,pass,chain"
SecRule ARGS "@rx y" "id:20,phase:2,pass"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	out, warns, err := processFileContent(path, map[string]struct{}{"20": {}}, false)
	require.NoError(t, err)
	require.NotContains(t, out, "id:10")
	require.NotContains(t, out, "id:20")
	require.True(t, strings.Contains(strings.Join(warns, ""), "SecRule chain"))
}
