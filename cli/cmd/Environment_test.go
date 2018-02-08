package cmd

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gopkg.in/h2non/gock.v1"
)

var expected_vars = map[string]string{
	"NAV_TRUSTSTORE_KEYSTOREALIAS": "app-key",
	"APP_DB_URL":                   "jdbc:oracle:thin:@//testdatabase.local:1521/app_db",
	"APP_DB_USERNAME":              "generic_database_username",
	"SOME_API_REST_URL":            "https://external-application.nais.preprod.local/rest/v1/very_useful_call",
}

func runCommand(t *testing.T, args []string) ([]byte, int) {
	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "app_db").
		MatchParam("application", "test-application").
		Reply(200).File("../../api/testdata/fasit_response_datasource.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "nav_truststore").
		MatchParam("application", "test-application").
		Reply(200).File("../../api/testdata/fasitTruststoreResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", "some_api").
		MatchParam("application", "test-application").
		Reply(200).File("../../api/testdata/fasit_response_restservice.json")

	gock.New("https://fasit.local").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).Body(bytes.NewReader([]byte("very secure certificate file :)")))

	// Since we're testing the output to console we need to save the default os.Stdout
	stdout := os.Stdout

	// To read from os.Stdout we replace it with a pipe
	r, w, err := os.Pipe()
	assert.Nil(t, err)
	os.Stdout = w

	reader := bufio.NewReader(r)

	// Execute the command
	RootCmd.SetArgs(args)
	err = environmentCommand.Execute()

	assert.Nil(t, err)

	// Extract the console output from our pipe
	commandResult := make([]byte, 512)
	commandResultLength, err := reader.Read(commandResult)

	assert.Nil(t, err)

	// When the command is done executing we revert to the original os.Stdout
	os.Stdout = stdout

	return commandResult, commandResultLength
}

func TestExportFormat(t *testing.T) {
	commandResult, commandResultLength := runCommand(t, []string{"env", "-u", "https://fasit.local", "-f", "../../api/testdata/nais_used_resources.yaml", "test-application"})

	assert.NotEqual(t, 0, commandResultLength)

	// Make sure the export format has a trailing newline
	assert.Equal(t, commandResult[commandResultLength-1], byte('\n'))

	// Remove the trailing newline and then split it by newlines
	resultStrings := strings.Split(string(commandResult[:commandResultLength-1]), "\n")

	assert.Equal(t, len(expected_vars), len(resultStrings))

	exportRegex, _ := regexp.Compile("export ([A-Z_]+)='(.+)'")
	for _, value := range resultStrings {
		matches := exportRegex.FindStringSubmatch(value)

		assert.NotNil(t, matches)

		assert.Contains(t, expected_vars, matches[1])
		assert.Equal(t, expected_vars[matches[1]], matches[2])
	}
}
func TestMultilineFormat(t *testing.T) {
	commandResult, commandResultLength := runCommand(t, []string{"env", "-u", "https://fasit.local", "-o", "multiline", "-f", "../../api/testdata/nais_used_resources.yaml", "test-application"})

	assert.NotEqual(t, 0, commandResultLength)

	// Make sure the export format has a trailing newline
	assert.Equal(t, commandResult[commandResultLength-1], byte('\n'))

	// Remove the trailing newline and then split it by newlines
	resultStrings := strings.Split(string(commandResult[:commandResultLength-1]), "\n")

	assert.Equal(t, len(expected_vars), len(resultStrings))

	exportRegex, _ := regexp.Compile("([A-Z_]+)='(.+)'")
	for _, value := range resultStrings {
		matches := exportRegex.FindStringSubmatch(value)

		assert.NotNil(t, matches)

		assert.Contains(t, expected_vars, matches[1])
		assert.Equal(t, expected_vars[matches[1]], matches[2])
	}
}

func TestDockerFormat(t *testing.T) {
	commandResult, commandResultLength := runCommand(t, []string{"env", "-u", "https://fasit.local", "-o", "docker", "-f", "../../api/testdata/nais_used_resources.yaml", "test-application"})

	assert.NotEqual(t, 0, commandResultLength)

	// Remove the first -e, so we can split by the environment flag
	resultStrings := strings.Split(string(commandResult[2:]), " -e")

	assert.Equal(t, len(expected_vars), len(resultStrings))

	exportRegex, _ := regexp.Compile("([A-Z_]+)='(.+)'") // The strings we got after splitting should have NAME='value' format
	for _, value := range resultStrings {
		matches := exportRegex.FindStringSubmatch(value)

		assert.NotNil(t, matches)

		assert.Contains(t, expected_vars, matches[1])
		assert.Equal(t, expected_vars[matches[1]], matches[2])
	}
}

func TestJavaFormat(t *testing.T) {
	commandResult, commandResultLength := runCommand(t, []string{"env", "-u", "https://fasit.local", "-o", "java", "-f", "../../api/testdata/nais_used_resources.yaml", "test-application"})

	assert.NotEqual(t, 0, commandResultLength)

	/* For java we're using -DNAME=variable, and in the current test data we don't expect any
	variables to have any spaces, so we can just split on space. If we later want to test
	with newlines we need to redo how we test induvidual variables. */
	resultStrings := strings.Split(string(commandResult[:]), " ")

	assert.Equal(t, len(expected_vars), len(resultStrings))

	exportRegex, _ := regexp.Compile("-D([A-Z_]+)='(.+)'")
	for _, value := range resultStrings {
		matches := exportRegex.FindStringSubmatch(value)

		assert.NotNil(t, matches)

		assert.Contains(t, expected_vars, matches[1])
		assert.Equal(t, expected_vars[matches[1]], matches[2])
	}
}
