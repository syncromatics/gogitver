package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/annymsmthd/gogitver/pkg/git"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	gogit "gopkg.in/src-d/go-git.v4"
)

var rootCmd = &cobra.Command{
	Use:   "gogitver",
	Short: "gogitver is a semver generator that uses git history",
	Long:  ``,
	Run:   runRoot,
}

var prereleaseCmd = &cobra.Command{
	Use:   "label",
	Short: "Gets the prerelease label, if any",
	Long:  ``,
	Run:   runPrerelease,
}

func init() {
	var cmds = [2]*cobra.Command{rootCmd, prereleaseCmd}
	for _, cmd := range cmds {
		cmd.Flags().String("path", ".", "the path to the git repository")
		cmd.Flags().String("settings", "./.gogitver.yaml", "the file that contains the settings")
		cmd.Flags().Bool("trim-branch-prefix", false, "Trim branch prefixes feature/ and hotfix/ from prerelease label")
	}

	rootCmd.Flags().Bool("forbid-behind-master", false, "error if the current branch's calculated version is behind the calculated version of refs/heads/master")

	rootCmd.AddCommand(prereleaseCmd)
}

// Execute gogitver
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getRepoAndSettings(cmd *cobra.Command) (*gogit.Repository, *git.Settings) {
	f := cmd.Flag("path")
	sf := cmd.Flag("settings")

	var s *git.Settings
	_, err := os.Stat(sf.Value.String())
	if sf.Changed || err == nil {
		r, err := os.Open(sf.Value.String())
		if err != nil {
			panic(errors.Wrap(err, "cannot open settings file"))
		}

		s, err = git.GetSettingsFromFile(r)
		if err != nil {
			panic(err)
		}
	} else {
		s = git.GetDefaultSettings()
	}

	r, err := gogit.PlainOpen(f.Value.String())
	if err != nil {
		panic(err)
	}

	return r, s
}

func getBoolFromFlag(cmd *cobra.Command, flagName string) bool {
	result, err := strconv.ParseBool(cmd.Flag(flagName).Value.String())
	if err != nil {
		result = false
	}
	return result
}

func getBranchSettings(cmd *cobra.Command) *git.BranchSettings {
	fbm := getBoolFromFlag(cmd, "forbid-behind-master")
	trimPrefix := getBoolFromFlag(cmd, "trim-branch-prefix")
	return &git.BranchSettings{
		ForbidBehindMaster: fbm,
		TrimBranchPrefix:   trimPrefix,
	}
}

func runRoot(cmd *cobra.Command, args []string) {
	r, s := getRepoAndSettings(cmd)

	branchSettings := getBranchSettings(cmd)
	version, err := git.GetCurrentVersion(r, s, branchSettings)
	if err != nil {
		panic(err)
	}

	fmt.Println(version)
}

func runPrerelease(cmd *cobra.Command, args []string) {
	r, s := getRepoAndSettings(cmd)
	trimPrefix := getBoolFromFlag(cmd, "trim-branch-prefix")
	branchSettings := &git.BranchSettings{
		TrimBranchPrefix: trimPrefix,
	}

	label, err := git.GetPrereleaseLabel(r, s, branchSettings)
	if err != nil {
		panic(err)
	}

	if label == "master" {
		label = ""
	}

	fmt.Println(label)
}
