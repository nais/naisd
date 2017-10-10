package cmd

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
)

const DEFAULT_NEXUS_URL = "http://maven.adeo.no/nexus/service/local/artifact/maven/content"

type NexusUploadRequest struct {
	Username string
	Password string
	Fields   NexusUploadFields
}

type NexusUploadFields struct {
	Repo       string
	Extension  string
	GroupId    string
	ArtifactId string
	Version    string
	Packaging  string
	File       string
}

func (r NexusUploadRequest) Validate() []error {
	required := map[string]*string{
		"Application": &r.Fields.ArtifactId,
		"Version":     &r.Fields.Version,
		"Username":    &r.Username,
		"Password":    &r.Password,
		"Repo":        &r.Fields.Repo,
		"Group":       &r.Fields.GroupId,
		"Packaging":   &r.Fields.Packaging,
		"Extension":   &r.Fields.Extension,
		"File":        &r.Fields.File,
	}

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required and is empty", key))
		}
	}

	return errs
}

func (r NexusUploadFields) WriteFields(w *multipart.Writer) error {
	if err := w.WriteField("r", r.Repo); err != nil {
		return err
	}
	if err := w.WriteField("hasPom", "false"); err != nil {
		return err
	}
	if err := w.WriteField("e", r.Extension); err != nil {
		return err
	}
	if err := w.WriteField("g", r.GroupId); err != nil {
		return err
	}
	if err := w.WriteField("a", r.ArtifactId); err != nil {
		return err
	}
	if err := w.WriteField("v", r.Version); err != nil {
		return err
	}
	if err := w.WriteField("p", r.Packaging); err != nil {
		return err
	}
	return nil
}

func readFile(filename string) (os.FileInfo, []byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	contents, err := ioutil.ReadAll(file)

	if err != nil {
		return nil, nil, err
	}

	fifo, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}

	return fifo, contents, nil
}

func (r NexusUploadFields) WriteFile(w *multipart.Writer) error {

	fifo, contents, err := readFile(r.File)
	if err != nil {
		return err
	}

	part, err := w.CreateFormFile("file", fifo.Name())
	if err != nil {
		return err
	}
	if _, err := part.Write(contents); err != nil {
		return err
	}

	return nil
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Uploads nais.yaml to Nexus",
	Long:  `Uploads nais.yaml to Nexus`,
	Run: func(cmd *cobra.Command, args []string) {
		nexusUploadRequest := NexusUploadRequest{
			Username: os.Getenv("NEXUS_USERNAME"),
			Password: os.Getenv("NEXUS_PASSWORD"),
			Fields: NexusUploadFields{
				Extension: "yaml",
				Packaging: "yaml",
			},
		}

		strings := map[string]*string{
			"group":    &nexusUploadRequest.Fields.GroupId,
			"app":      &nexusUploadRequest.Fields.ArtifactId,
			"version":  &nexusUploadRequest.Fields.Version,
			"username": &nexusUploadRequest.Username,
			"password": &nexusUploadRequest.Password,
			"file":     &nexusUploadRequest.Fields.File,
			"repo":     &nexusUploadRequest.Fields.Repo,
		}

		for key, pointer := range strings {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("Error when getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		if err := nexusUploadRequest.Validate(); err != nil {
			fmt.Printf("NexusUploadRequest is not valid: %v\n", err)
			os.Exit(1)
		}

		requestBody := new(bytes.Buffer)

		writer := multipart.NewWriter(requestBody)

		if err := nexusUploadRequest.Fields.WriteFields(writer); err != nil {
			fmt.Printf("Failed to write fields: %v\n", err)
			os.Exit(1)
		}

		if err := nexusUploadRequest.Fields.WriteFile(writer); err != nil {
			fmt.Printf("Failed to write file: %v\n", err)
			os.Exit(1)
		}

		if err := writer.Close(); err != nil {
			fmt.Printf("Failed to close writer: %v\n", err)
			os.Exit(1)
		}

		nexusUrl := DEFAULT_NEXUS_URL
		if url, ok := os.LookupEnv("NEXUS_URL"); ok {
			nexusUrl = url
		}

		req, err := http.NewRequest("POST", nexusUrl, requestBody)
		if err != nil {
			fmt.Printf("Failed to create http request: %v\n", err)
			os.Exit(1)
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.SetBasicAuth(nexusUploadRequest.Username, nexusUploadRequest.Password)

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			fmt.Printf("Error while sending POST request: %v\n", err)
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
	uploadCmd.Flags().StringP("repo", "r", "m2internal", "nexus repo")
	uploadCmd.Flags().StringP("group", "g", "nais", "nexus group")
	uploadCmd.Flags().StringP("file", "f", "nais.yaml", "path to nais.yaml")
	uploadCmd.Flags().StringP("username", "u", "", "the username")
	uploadCmd.Flags().StringP("password", "p", "", "the password")
}
