package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

const DEFAULT_NEXUS_URL = "https://repo.adeo.no/repository/raw"

type NexusUploadRequest struct {
	Username string
	Password string
	App      string
	Version  string
	File     string
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Uploads nais.yaml to Nexus",
	Long:  `Uploads nais.yaml to Nexus`,
	Run: func(cmd *cobra.Command, args []string) {
		nexusUploadRequest := NexusUploadRequest{
			Username: os.Getenv("NEXUS_USERNAME"),
			Password: os.Getenv("NEXUS_PASSWORD"),
		}

		opts := map[string]*string{
			"app":      &nexusUploadRequest.App,
			"version":  &nexusUploadRequest.Version,
			"username": &nexusUploadRequest.Username,
			"password": &nexusUploadRequest.Password,
			"file":     &nexusUploadRequest.File,
		}

		for key, pointer := range opts {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("failed getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		file, err := os.Open(nexusUploadRequest.File)
		if err != nil {
			fmt.Printf("failed to open file: %v\n", err)
			os.Exit(1)
		}

		nexusUrl := DEFAULT_NEXUS_URL
		if url, ok := os.LookupEnv("NEXUS_URL"); ok {
			nexusUrl = strings.TrimRight(url, "/")
		}

		nexusUrl = fmt.Sprintf("%s/nais/%s/%s/nais.yaml", nexusUrl, nexusUploadRequest.App, nexusUploadRequest.Version)

		req, err := http.NewRequest("PUT", nexusUrl, file)
		if err != nil {
			fmt.Printf("failed to create http request: %v\n", err)
			os.Exit(1)
		}

		req.SetBasicAuth(nexusUploadRequest.Username, nexusUploadRequest.Password)

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			fmt.Printf("error while uploading file: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Body:", string(body))

		if resp.StatusCode > 299 {
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringP("app", "a", "", "name of your app")
	uploadCmd.Flags().StringP("version", "v", "", "version you want to upload")
	uploadCmd.Flags().StringP("file", "f", "nais.yaml", "path to nais.yaml")
	uploadCmd.Flags().StringP("username", "u", "", "the username")
	uploadCmd.Flags().StringP("password", "p", "", "the password")
}
