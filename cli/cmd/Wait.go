package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"time"
)

func waitForDeploy(url string) error {
	for {
		resp, err := http.Get(url)

		if err != nil {
			return err
		}

		if resp.StatusCode == 200 {
			break
		}

		if resp.StatusCode != 202 {
			return fmt.Errorf("Deploy failed: %d\n", resp.StatusCode)
		}

		// do nothing, continue loop
		time.Sleep(1000 * time.Millisecond)
	}
	return nil
}

var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Waits for deploy",
	Long:  `Waits for deploy`,
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

		var cluster, app, namespace string
		strings := map[string]*string{
			"app":       &app,
			"namespace": &namespace,
			"cluster":   &cluster,
		}

		for key, pointer := range strings {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("Error when getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		if len(app) == 0 {
			fmt.Println("Application cannot be empty")
			os.Exit(1)
		}

		cluster, exists := clusters[cluster]
		if !exists {
			fmt.Print("Cluster is not valid, please choose one of: ")
			for key := range clusters {
				fmt.Printf("%s, ", key)
			}
			fmt.Print("\n")
			os.Exit(1)
		}

		clusterUrl := os.Getenv("NAIS_CLUSTER_URL")
		if len(clusterUrl) == 0 {
			clusterUrl = "https://daemon." + cluster
		}

		if err := waitForDeploy(clusterUrl + STATUS_ENDPOINT + "/" + namespace + "/" + app); err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}

		fmt.Println("Deploy successful")
	},
}

func init() {
	RootCmd.AddCommand(waitCmd)

	waitCmd.Flags().StringP("app", "a", "", "name of your app")
	waitCmd.Flags().StringP("cluster", "c", "preprod-fss", "the cluster you want to deploy to")
	waitCmd.Flags().StringP("namespace", "n", "default", "the kubernetes namespace")
}
