package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/nais/naisd/api/app"
	"github.com/nais/naisd/api/naisrequest"
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

func GenerateDeployMessage(spec app.Spec, deploymentRequest *naisrequest.Deploy, clusterName *string) ([]byte, error) {
	output := fmt.Sprintf("naisd.deployment,application=%s,clusterName=%s,namespace=%s version=\"%s\" %d", spec.Application, *clusterName, spec.Namespace(), deploymentRequest.Version, time.Now().UnixNano())
	m := message{"naisd.deployment", "metric", []string{"events_nano"}, output}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("can't marshal message for Sensu. Message was: %s\nError was: %s", m, err)
	}

	return b, nil
}

func sendMessage(message []byte) error {
	conn, err := net.Dial("tcp", defaultSensuHost)
	if err != nil {
		return fmt.Errorf("problem connecting to sensu on %s\nError was: %s", defaultSensuHost, err)
	}

	defer conn.Close()

	conn.Write(message)
	conn.Write([]byte(stopCharacter))

	buff := make([]byte, 1024)
	_, err = conn.Read(buff)
	if err != nil {
		return fmt.Errorf("problem reading response from sensu\nError was: %s", err)
	}

	i := bytes.Index(buff, []byte("\x00"))
	if string(buff[:i]) != "ok" {
		return fmt.Errorf("sensu repsonded with something other than 'ok'. Response was: '%s'", string(buff))
	}

	glog.Info("Notified Sensu about deployment, sent the following message: ", string(message))
	return nil
}

func NotifySensuAboutDeploy(spec app.Spec, deploymentRequest *naisrequest.Deploy, clusterName *string) {
	message, err := GenerateDeployMessage(spec, deploymentRequest, clusterName)
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
