package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"net"
	"time"
)

const (
	defaultSensuHost = "sensu.nais:3030"
	stopCharacter    = "\n"
)

type message struct {
	Name        string   `json:"name"`
	MessageType string   `json:"type"`
	Handlers    []string `json:"handlers"`
	Output      string   `json:"output"`
}

func unixTimeInNano() int64 {
	return time.Now().UnixNano()
}

func GenerateDeployMessage(deploymentRequest *NaisDeploymentRequest, clusterName *string) ([]byte, error) {
	timestamp := unixTimeInNano()
	output := fmt.Sprintf("naisd.deployment,application=%s,clusterName=%s,namespace=%s version=\"%s\" %d", deploymentRequest.Application, *clusterName, deploymentRequest.Namespace, deploymentRequest.Version, timestamp)
	m := message{"naisd.deployment", "metric", []string{"events_nano"}, output}

	b, err := json.Marshal(m)
	if err != nil {
		errMsg := fmt.Sprintf("Can't marshal message for Sensu. Message was: %s\nError was: %s", m, err)
		return nil, errors.New(errMsg)
	}

	return b, nil
}

func sendMessage(message []byte) error {
	conn, err := net.Dial("tcp", defaultSensuHost)
	if err != nil {
		errMsg := fmt.Sprintf("Problem connecting to sensu on %s\nError was: %s", defaultSensuHost, err)
		return errors.New(errMsg)
	}

	defer conn.Close()

	conn.Write(message)
	conn.Write([]byte(stopCharacter))

	buff := make([]byte, 1024)
	_, err = conn.Read(buff)
	if err != nil {
		errMsg := fmt.Sprintf("Problem reading response from sensu\nError was: %s", err)
		return errors.New(errMsg)
	}

	i := bytes.Index(buff, []byte("\x00"))
	if string(buff[:i]) != "ok" {
		errMsg := fmt.Sprintf("Sensu repsonded with something other than 'ok'. Response was: '%s'", string(buff))
		return errors.New(errMsg)
	}

	glog.Info("Notified Sensu about deployment, sent the following message: ", string(message))
	return nil
}

func NotifySensuAboutDeploy(deploymentRequest *NaisDeploymentRequest, clusterName *string) {
	message, err := GenerateDeployMessage(deploymentRequest, clusterName)
	if err != nil {
		glog.Errorln(err)
		return
	}

	err = sendMessage(message)
	if err != nil {
		glog.Errorln(err)
		return
	}
}
