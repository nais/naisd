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

		clusterUrl, err := getClusterUrl(cluster)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		start := time.Now()
		if err := waitForDeploy(clusterUrl + StatusEndpoint + "/" + namespace + "/" + app); err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		elapsed := time.Since(start)
		fmt.Printf("Deploy successful, took %v\n", elapsed)
	},
}

func init() {
	RootCmd.AddCommand(waitCmd)

	waitCmd.Flags().StringP("app", "a", "", "name of your app")
	waitCmd.Flags().StringP("cluster", "c", "", "the cluster you want to deploy to")
	waitCmd.Flags().StringP("namespace", "n", "default", "the kubernetes namespace")
}
