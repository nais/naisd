package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nais/naisd/api/constant"
	"github.com/nais/naisd/api/naisrequest"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"syscall"
	"time"
)

const DeployEndpoint = "/deploy"
const StatusEndpoint = "/deploystatus"
const DefaultCluster = "preprod-fss"

var clustersDict = map[string]string{
	"ci":           "nais-ci.devillo.no",
	"nais-dev":     "nais.devillo.no",
	"preprod-fss":  "nais.preprod.local",
	"prod-fss":     "nais.adeo.no",
	"preprod-iapp": "nais-iapp.preprod.local",
	"prod-iapp":    "nais-iapp.adeo.no",
	"preprod-sbs":  "nais.oera-q.local",
	"prod-sbs":     "nais.oera.no",
}

func validateCluster(cluster string) (string, error) {
	url, exists := clustersDict[cluster]
	if exists {
		return url, nil
	}

	errmsg := fmt.Sprint("Cluster is not valid, please choose one of: ")
	for key := range clustersDict {
		errmsg = errmsg + fmt.Sprintf("%s, ", key)
	}

	return "", errors.New(errmsg)
}

func getClusterUrl(cluster string) (string, error) {
	urlEnv := os.Getenv("NAIS_CLUSTER_URL")

	if len(cluster) == 0 {
		if len(urlEnv) > 0 {
			return urlEnv, nil
		} else {
			cluster = DefaultCluster
		}
	}

	url, err := validateCluster(cluster)
	if err != nil {
		return "", err
	}

	return "https://daemon." + url, nil
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys your application",
	Long:  `Deploys your application`,
	Run: func(cmd *cobra.Command, args []string) {
		deployRequest := naisrequest.Deploy{
			FasitUsername: os.Getenv("FASIT_USERNAME"),
			FasitPassword: os.Getenv("FASIT_PASSWORD"),
		}

		var cluster string
		strings := map[string]*string{
			"app":               &deployRequest.Application,
			"version":           &deployRequest.Version,
			"zone":              &deployRequest.Zone,
			"namespace":         &deployRequest.Namespace,
			"fasit-environment": &deployRequest.FasitEnvironment,
			"fasit-username":    &deployRequest.FasitUsername,
			"fasit-password":    &deployRequest.FasitPassword,
			"manifest-url":      &deployRequest.ManifestUrl,
			"cluster":           &cluster,
		}

		for key, pointer := range strings {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("Error when getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		deployRequest.SkipFasit, _ = cmd.Flags().GetBool("skip-fasit")

		if !deployRequest.SkipFasit {
			if deployRequest.FasitUsername == "" {
				currentUser, err := user.Current()
				if err != nil {
					fmt.Fprintln(os.Stderr, "Unable resolve a username, please specify FASIT_USERNAME")
					os.Exit(1)
				}
				deployRequest.FasitUsername = currentUser.Username
			}

			if deployRequest.FasitPassword == "" {
				fmt.Fprintf(os.Stderr, "Enter password for %s: ", deployRequest.FasitUsername)
				passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error occurred while trying to read password from stdin\n")
					os.Exit(1)
				}
				deployRequest.FasitPassword = string(passwordBytes)
				fmt.Fprintln(os.Stderr)
			}
		}

		if err := deployRequest.Validate(); err != nil {
			fmt.Printf("DeploymentRequest is not valid: %v\n", err)
			os.Exit(1)
		}

		clusterUrl, err := getClusterUrl(cluster)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		jsonStr, err := json.Marshal(deployRequest)

		if err != nil {
			fmt.Printf("Error while marshalling JSON: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(jsonStr))

		resp, err := http.Post(clusterUrl+DeployEndpoint, "application/json", bytes.NewBuffer(jsonStr))

		if err != nil {
			fmt.Printf("Error while POSTing to API: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Body:", string(body))

		if resp.StatusCode > 299 {
			os.Exit(1)
		}

		if wait, err := cmd.Flags().GetBool("wait"); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else if wait {
			start := time.Now()
			if err := waitForDeploy(clusterUrl + StatusEndpoint + "/" + deployRequest.Namespace + "/" + deployRequest.Application); err != nil {
				fmt.Printf("%v\n", err)
				os.Exit(1)
			}
			elapsed := time.Since(start)
			fmt.Printf("Deploy successful, took %v\n", elapsed)
		}
	},
}

func init() {
	RootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringP("app", "a", "", "name of your app")
	deployCmd.Flags().StringP("version", "v", "", "version you want to deploy")
	deployCmd.Flags().StringP("cluster", "c", "", "the cluster you want to deploy to")
	deployCmd.Flags().StringP("fasit-environment", "e", "q0", "environment you want to use")
	deployCmd.Flags().StringP("zone", "z", constant.ZONE_FSS, "the zone the app will be in")
	deployCmd.Flags().StringP("namespace", "n", "default", "the kubernetes namespace")
	deployCmd.Flags().StringP("fasit-username", "u", "", "the username")
	deployCmd.Flags().StringP("fasit-password", "p", "", "the password")
	deployCmd.Flags().StringP("manifest-url", "m", "", "alternative URL to the nais manifest")
	deployCmd.Flags().Bool("wait", false, "whether to wait until the deploy has succeeded (or failed)")
	deployCmd.Flags().Bool("skip-fasit", false, "whether to skip interaction with fasit")
}
