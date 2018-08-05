package cmd

import (
	"fmt"
	"os"

	"github.com/annymsMthd/gogitver/pkg/git"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gogitver",
	Short: "gogitver is a semver generator that uses git history",
	Long:  ``,
	Run:   runRoot,
}

func init() {
	rootCmd.Flags().String("path", ".", "the path to the git repostiory")
}

// Execute gogitver
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) {
	f := cmd.Flag("path")

	version, err := git.GetCurrentVerion(f.Value.String())
	if err != nil {
		panic(err)
	}

	fmt.Println(version)
}
