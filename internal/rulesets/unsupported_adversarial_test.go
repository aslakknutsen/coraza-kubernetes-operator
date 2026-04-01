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

package rulesets

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckUnsupportedRules_Adversarial_CommentLineBareIDIgnored(t *testing.T) {
	rules := "# id:950150\nSecRuleEngine On\n"
	assert.Nil(t, CheckUnsupportedRules(rules))
}

func TestCheckUnsupportedRules_Adversarial_SingleQuotedID(t *testing.T) {
	found := CheckUnsupportedRules(`SecRule REQUEST_URI "@rx x" "id:'950150',phase:1,pass"`)
	require.Len(t, found, 1)
	assert.Equal(t, 950150, found[0].ID)
}

func TestCheckUnsupportedRules_Adversarial_DoubleQuotedID(t *testing.T) {
	rules := fmt.Sprintf(`SecRule REQUEST_URI "@rx x" "id:%s950150%s,phase:1,pass"`, `"`, `"`)
	found := CheckUnsupportedRules(rules)
	require.Len(t, found, 1)
	assert.Equal(t, 950150, found[0].ID)
}

func TestCheckUnsupportedRules_Adversarial_EmptyString(t *testing.T) {
	assert.Nil(t, CheckUnsupportedRules(""))
}

func TestCheckUnsupportedRules_Adversarial_OnlyComments(t *testing.T) {
	rules := "# SecRuleEngine On\n   # another\n"
	assert.Nil(t, CheckUnsupportedRules(rules))
}

func TestCheckUnsupportedRules_Adversarial_DuplicateIDsOnce(t *testing.T) {
	rules := secRuleWithID(950150) + "\n" + secRuleWithID(950150) + "\n" + secRuleWithID(950150)
	found := CheckUnsupportedRules(rules)
	require.Len(t, found, 1)
	assert.Equal(t, 950150, found[0].ID)
}

func TestFormatUnsupportedMessage_Adversarial_EmptySlice(t *testing.T) {
	assert.Equal(t, "", FormatUnsupportedMessage([]UnsupportedRule{}))
}

func TestIncompatibleRuleIDs_Adversarial_SortedNonEmpty(t *testing.T) {
	ids := IncompatibleRuleIDs()
	require.NotEmpty(t, ids)
	for i := 1; i < len(ids); i++ {
		assert.Less(t, ids[i-1], ids[i], "must be strictly sorted ascending")
	}
}

func TestRedundantRuleIDs_Adversarial_SortedNonEmpty(t *testing.T) {
	ids := RedundantRuleIDs()
	require.NotEmpty(t, ids)
	for i := 1; i < len(ids); i++ {
		assert.Less(t, ids[i-1], ids[i], "must be strictly sorted ascending")
	}
}

func TestStripCommentLines_Adversarial_WhitespaceOnlyLines(t *testing.T) {
	in := "  \t  \nSecRule X \"@rx a\" \"id:950150,pass\"\n\n"
	out := stripCommentLines(in)
	assert.Contains(t, out, "id:950150")
	assert.NotContains(t, out, "\n\n\n")
}

func TestStripCommentLines_Adversarial_MixedTabsSpacesBeforeHash(t *testing.T) {
	in := "SecRule A \"@rx b\" \"id:1,pass\"\n \t # id:950150\nSecRule C \"@rx d\" \"id:922110,pass\""
	out := stripCommentLines(in)
	assert.Contains(t, out, "id:1")
	assert.NotContains(t, out, "id:950150")
	assert.Contains(t, out, "id:922110")
}
