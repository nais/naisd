package cmd

import (
	"fmt"
	"github.com/nais/naisd/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates nais.yaml",
	Long:  `Validates nais.yaml`,
	Run: func(cmd *cobra.Command, args []string) {

		file, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Printf("Error when getting flag: file. %v", err)
			os.Exit(1)
		}
		output, err := cmd.Flags().GetBool("output")
		if err != nil {
			fmt.Printf("Error when getting flag: output. %v", err)
			os.Exit(1)
		}

		naisYaml, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("Could not read file: "+file+". %v", err)
			os.Exit(1)
		}

		fmt.Println("Validating the file: " + file)

		var manifest api.NaisManifest

		if err := yaml.Unmarshal(naisYaml, &manifest); err != nil {
			fmt.Printf("Error while unmarshalling yaml. %v", err)
			os.Exit(1)
		}

		if err := api.AddDefaultManifestValues(&manifest, "appName"); err != nil {
			fmt.Printf("Error while adding default values yaml. %v", err)
			os.Exit(1)
		}

		if output {
			conf, _ := yaml.Marshal(manifest)
			fmt.Println(string(conf))
		}

		validationErrors := api.ValidateManifest(manifest)
		if len(validationErrors.Errors) != 0 {
			fmt.Println("Found errors while validating " + file)
			fmt.Printf("%v", validationErrors)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "nais.yaml", "path to manifest")
	validateCmd.Flags().BoolP("output", "o", false, "prints full manifest including defaults")
}
