package git

import (
	"io"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Settings provides the regex patterns used for version bumping
type Settings struct {
	MajorPattern string `yaml:"major-version-bump-message"`
	MinorPattern string `yaml:"minor-version-bump-message"`
	PatchPattern string `yaml:"patch-version-bump-message"`
}

// GetSettingsFromFile provides a settings object by parsing the yaml from the file provided
func GetSettingsFromFile(file io.Reader) (*Settings, error) {
	s := Settings{}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "read bytes from file failed")
	}

	err = yaml.Unmarshal(fileBytes, &s)
	if err != nil {
		return nil, errors.Wrap(err, "read yaml from file failed")
	}

	return &s, nil
}

// GetDefaultSettings returns the default settings
func GetDefaultSettings() *Settings {
	return &Settings{
		MajorPattern: "\\+semver:\\s?(breaking|major)",
		MinorPattern: "\\+semver:\\s?(feature|minor)",
		PatchPattern: "\\+semver:\\s?(fix|patch)",
	}
}
