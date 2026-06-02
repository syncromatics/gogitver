package git_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syncromatics/gogitver/pkg/git"
)

func TestSettingsParse(t *testing.T) {
	testString := `
major-version-bump-message: '\+semver:\s?(breaking|major)'
minor-version-bump-message: '\+semver:\s?(feature|minor)'
patch-version-bump-message: '\+semver:\s?(fix|patch)'
`

	b := []byte(testString)
	r := bytes.NewReader(b)

	s, err := git.GetSettingsFromFile(r)
	require.NoError(t, err)

	assert.Equal(t, "\\+semver:\\s?(breaking|major)", s.MajorPattern)

	assert.Equal(t, "\\+semver:\\s?(feature|minor)", s.MinorPattern)

	assert.Equal(t, "\\+semver:\\s?(fix|patch)", s.PatchPattern)
}
