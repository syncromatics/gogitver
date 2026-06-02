package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/syncromatics/gogitver/pkg/git"

	gogit "github.com/go-git/go-git/v5"
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
		cmd.Flags().String("primary-branch", "", "the primary branch used as the stable version baseline")
		cmd.Flags().Bool("trim-branch-prefix", false, "Trim branch prefixes feature/ and hotfix/ from prerelease label")
		cmd.Flags().BoolP("verbose", "v", false, "Show information about how the version was calculated")
	}

	rootCmd.Flags().Bool("forbid-behind-primary", false, "error if the current branch's calculated version is behind the calculated version of the primary branch")
	rootCmd.Flags().Bool("forbid-behind-master", false, "deprecated alias for --forbid-behind-primary")

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
	flag := cmd.Flag(flagName)
	if flag == nil {
		return false
	}

	result, err := strconv.ParseBool(flag.Value.String())
	if err != nil {
		result = false
	}
	return result
}

func getBranchSettings(cmd *cobra.Command) *git.BranchSettings {
	fbm := getBoolFromFlag(cmd, "forbid-behind-master")
	fbp := getBoolFromFlag(cmd, "forbid-behind-primary")
	trimPrefix := getBoolFromFlag(cmd, "trim-branch-prefix")
	primaryBranch := ""
	if flag := cmd.Flag("primary-branch"); flag != nil {
		primaryBranch = flag.Value.String()
	}
	return &git.BranchSettings{
		ForbidBehindMaster:  fbm,
		ForbidBehindPrimary: fbp,
		TrimBranchPrefix:    trimPrefix,
		PrimaryBranch:       primaryBranch,
	}
}

func runRoot(cmd *cobra.Command, args []string) {
	r, s := getRepoAndSettings(cmd)
	v := getBoolFromFlag(cmd, "verbose")

	if v {
		log.SetFlags(0)
	}

	branchSettings := getBranchSettings(cmd)
	version, err := git.GetCurrentVersion(r, s, branchSettings, v)
	if err != nil {
		panic(err)
	}

	fmt.Println(version)
}

func runPrerelease(cmd *cobra.Command, args []string) {
	r, s := getRepoAndSettings(cmd)
	branchSettings := getBranchSettings(cmd)

	label, err := git.GetPrereleaseLabel(r, s, branchSettings)
	if err != nil {
		panic(err)
	}

	primaryBranch, err := git.GetPrimaryBranchName(r, branchSettings)
	if err == nil && label == primaryBranch {
		label = ""
	}

	fmt.Println(label)
}
