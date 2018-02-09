package cmd

import (
	"fmt"
	"github.com/nais/naisd/api/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints version",
	Long:  `Prints version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version %v, revision %v\n", version.Version, version.Revision)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
