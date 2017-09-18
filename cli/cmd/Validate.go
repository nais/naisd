package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates nais.yml",
	Long: `Validates nais.yml`,
	Run: func(cmd *cobra.Command, args []string) {

		file, _ := cmd.Flags().GetString("file")

		fmt.Println("we are validating the file: " + file)
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "nais.yml", "path to appconfig")
}
