package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nais/naisd/api"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys your application",
	Long:  `Deploys your application`,
	Run: func(cmd *cobra.Command, args []string) {

		clusters := map[string]string{
			"ci":           "nais-ci.devillo.no",
			"nais-dev":     "nais.devillo.no",
			"preprod-fss":  "nais.preprod.local",
			"prod-fss":     "nais.adeo.no",
			"preprod-iapp": "nais-iapp.preprod.local",
			"prod-iapp":    "nais-iapp.adeo.no",
			"preprod-sbs":  "nais.oera-q.local",
			"prod-sbs":     "nais.oera.no",
		}

		deployRequest := api.NaisDeploymentRequest{
			Username: os.Getenv("NAIS_USERNAME"),
			Password: os.Getenv("NAIS_PASSWORD"),
		}

		var cluster string
		strings := map[string]*string{
			"app":         &deployRequest.Application,
			"version":     &deployRequest.Version,
			"environment": &deployRequest.Environment,
			"zone":        &deployRequest.Zone,
			"namespace":   &deployRequest.Namespace,
			"username":    &deployRequest.Username,
			"password":    &deployRequest.Password,
			"cluster":     &cluster,
		}

		for key, pointer := range strings {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("Error when getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		if err := deployRequest.Validate(); err != nil {
			fmt.Printf("DeploymentRequest is not valid: %v\n", err)
			os.Exit(1)
		}

		cluster = clusters[cluster]
		url := "https://daemon." + cluster + "/deploy"

		jsonStr, err := json.Marshal(deployRequest)

		if err != nil {
			fmt.Printf("Error while marshalling JSON: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(jsonStr))

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonStr))

		if err != nil {
			fmt.Printf("Error while POSTing to API: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Body:", string(body))
	},
}

func init() {
	RootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringP("app", "a", "", "name of your app")
	deployCmd.Flags().StringP("version", "v", "", "version you want to deploy")
	deployCmd.Flags().StringP("cluster", "c", "preprod-fss", "the cluster you want to deploy to")
	deployCmd.Flags().StringP("environment", "e", "t0", "environment you want to use")
	deployCmd.Flags().StringP("zone", "z", "fss", "the zone the app will be in")
	deployCmd.Flags().StringP("namespace", "n", "default", "the kubernetes namespace")
	deployCmd.Flags().StringP("username", "u", "", "the username")
	deployCmd.Flags().StringP("password", "p", "", "the password")
}
