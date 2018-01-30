package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nais/naisd/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var environmentCommand = &cobra.Command{
	Short: "Update environment variables",
	Long:  `Command to update environment variables with values fetched from the defined fasit resource`,
	Use:   `env [flags] <application_name>`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Help()
			os.Exit(1)
		}

		application := args[0]
		username := os.Getenv("FASIT_USERNAME")
		password := os.Getenv("FASIT_PASSWORD")

		inline := true

		stringFormat := "%v='%s'"

		naisFileName, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when getting flag: file. %v\n", err)
			os.Exit(1)
		}

		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while getting flag: output. %v\n", err)
			os.Exit(1)
		}

		zone, err := cmd.Flags().GetString("zone")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while getting flag: zone. %v\n", err)
			os.Exit(1)
		}

		environment, err := cmd.Flags().GetString("environment")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while getting flag: environment. %v\n", err)
			os.Exit(1)
		}

		fasitUrl, err := cmd.Flags().GetString("fasit-url")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while getting flag: fasit-url. %v\n", err)
			os.Exit(1)
		}

		switch outputFormat {
		case "export":
			stringFormat = "export " + stringFormat
			inline = false
		case "java":
			stringFormat = "-D" + stringFormat
		case "docker":
			stringFormat = "-e " + stringFormat
		case "multiline":
			inline = false
		case "inline":
			// Defaults works fine
		default:
			fmt.Fprintf(os.Stderr, "Invalid output format %s\n", outputFormat)
			os.Exit(1)
		}

		file, err := ioutil.ReadFile(naisFileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to read the file %v: %v\n", naisFileName, err)
			os.Exit(1)
		}

		var manifest api.NaisManifest

		if err := yaml.Unmarshal(file, &manifest); err != nil {
			fmt.Fprintf(os.Stderr, "Error while unmarshalling yaml. %v", err)
			os.Exit(1)
		}

		fasit := api.FasitClient{
			Username: username,
			Password: password,
			FasitUrl: fasitUrl,
		}

		vars, err := api.FetchFasitResources(fasit, application, environment, zone, manifest.FasitResources.Used)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to contact Fasit. %v\n", err)
			os.Exit(1)
		}

		formattedVars := make([]string, 0)

		for _, resource := range vars {
			for key, val := range resource.GetProperties() {
				environmentVariable := resource.ToEnvironmentVariable(key)
				formattedVars = append(formattedVars, fmt.Sprintf(stringFormat, environmentVariable, val))
			}
		}

		joinChar := "\n"
		if inline {
			joinChar = " "
		}

		resultString := strings.Join(formattedVars, joinChar)
		fmt.Printf(resultString)

		if !inline {
			fmt.Println()
		}
	},
}

func init() {
	RootCmd.AddCommand(environmentCommand)
	environmentCommand.Flags().StringP("output", "o", "export", `How to format the output, valid options`)
	environmentCommand.Flags().StringP("file", "f", "nais.yaml", `Define the file to parse`)
	environmentCommand.Flags().StringP("zone", "z", "fss", `Which zone the application is deployed in`)
	environmentCommand.Flags().StringP("environment", "e", "t0", `Which fasit environment to fetch variables from`)
	environmentCommand.Flags().StringP("fasit-url", "u", "https://fasit.adeo.no", `Set fasit url`)
}
