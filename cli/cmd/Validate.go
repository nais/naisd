package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
	"github.com/nais/naisd/api"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates nais.yml",
	Long:  `Validates nais.yml`,
	Run: func(cmd *cobra.Command, args []string) {

		file, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Printf("Error when getting flag: file. %v", err)
			return
		}
		output, err := cmd.Flags().GetBool("output")
		if err != nil {
			fmt.Printf("Error when getting flag: output. %v", err)
			return
		}

		naisYaml, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("Could not read file: "+file+". %v", err)
		}

		fmt.Println("Validating the file: " + file)

		var appConfig api.NaisAppConfig

		if err := yaml.Unmarshal(naisYaml, &appConfig); err != nil {
			fmt.Printf("Error while unmarshalling yaml. %v", err)
		}

		if err := api.AddDefaultAppconfigValues(&appConfig, "appName"); err != nil {
			fmt.Printf("Error while adding default values yaml. %v", err)
		}

		if output {
			//yaml.Marshal(appConfig)
			conf, _  := yaml.Marshal(appConfig)
			fmt.Println(string(conf))
		}

		validationErrors := api.ValidateAppConfig(appConfig);
		if len(validationErrors.Errors) != 0 {
			fmt.Println("Found errors while validating " + file)
			fmt.Printf("%v", validationErrors)
		}
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "nais.yml", "path to appconfig")
	validateCmd.Flags().BoolP("output", "o", false, "prints full appconfig including defaults if tr")
}
