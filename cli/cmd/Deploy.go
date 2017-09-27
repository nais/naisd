package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/imdario/mergo"
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

		if app, err := cmd.Flags().GetString("app"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "app", err)
			os.Exit(1)
		} else {
			deployRequest.Application = app
		}

		if version, err := cmd.Flags().GetString("version"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "version", err)
			os.Exit(1)
		} else {
			deployRequest.Version = version
		}

		if environment, err := cmd.Flags().GetString("environment"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "environment", err)
			os.Exit(1)
		} else {
			deployRequest.Environment = environment
		}

		if zone, err := cmd.Flags().GetString("zone"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "zone", err)
			os.Exit(1)
		} else {
			deployRequest.Zone = zone
		}

		if namespace, err := cmd.Flags().GetString("namespace"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "namespace", err)
			os.Exit(1)
		} else {
			deployRequest.Namespace = namespace
		}

		if username, err := cmd.Flags().GetString("username"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "username", err)
			os.Exit(1)
		} else if len(username) > 0 {
			deployRequest.Username = username
		}

		if password, err := cmd.Flags().GetString("password"); err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "password", err)
			os.Exit(1)
		} else if len(password) > 0 {
			deployRequest.Password = password
		}

		cluster, err := cmd.Flags().GetString("cluster")

		if err != nil {
			fmt.Printf("Error when getting flag: %s. %v", "environment", err)
			os.Exit(1)
		}

		cluster = clusters[cluster]

		url := "https://daemon." + cluster + "/deploy"

		err = mergo.Merge(&deployRequest, api.NaisDeploymentRequest{
			Environment: "t0",
			Zone:        "fss",
			Namespace:   "default",
		})

		if err != nil {
			fmt.Printf("Error while merging default config: %v\n", err)
			os.Exit(1)
		}

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
	deployCmd.Flags().StringP("cluster", "c", "nais-dev", "the cluster you want to deploy to")
	deployCmd.Flags().StringP("environment", "e", "t0", "environment you want to use")
	deployCmd.Flags().StringP("zone", "z", "fss", "the zone the app will be in")
	deployCmd.Flags().StringP("namespace", "n", "default", "the kubernetes namespace")
	deployCmd.Flags().StringP("username", "u", "", "the username")
	deployCmd.Flags().StringP("password", "p", "", "the password")
}
