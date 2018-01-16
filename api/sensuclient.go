package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
	"github.com/golang/glog"
	"bytes"
)

const (
	DEFAULT_SENSU_HOST = "sensu.nais:3030"
	STOP_CHARACTER = "\r\n\r\n"
)

type Message struct {
	Name string `json:"name"`
	Message_type string `json:"type"`
	Handlers []string `json:"handlers"`
	Output string `json:"output"`
}

func unixTimeInNano() int64 {
	return time.Now().UnixNano()
}

func GenerateDeployMessage(application *string, clusterName *string, namespace *string, version *string) ([]byte, error) {
	timestamp := unixTimeInNano()
	output := fmt.Sprintf("naisd.deployment,application=%s,clusterName=%s,namespace=%s version=\"%s\" %d", *application, *clusterName, *namespace, *version, timestamp)
	m := Message{"naisd.deployment", "metric", []string{"events_nano"}, output}
	b, err := json.Marshal(m)

	if err != nil {
		errMsg := fmt.Sprintf("Can't marshal message for Sensu. Message was: %s\nError was: %s", m, err)
		return nil, errors.New(errMsg)
	}

	return b, nil
}

func sendMessage(message []byte) error {
	conn, err := net.Dial("tcp", DEFAULT_SENSU_HOST)

	if err != nil {
		errMsg := fmt.Sprintf("Problem connecting to sensu on %s\nError was: %s", DEFAULT_SENSU_HOST, err)
		return errors.New(errMsg)
	}

	defer conn.Close()

	conn.Write(message)
	conn.Write([]byte(STOP_CHARACTER))

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
	} else {
		glog.Info("Notified Sensu about deployment, sent the following message: ", string(message))
	}

	return nil
}

func NotifyGrafanaAboutDeploy(application *string, clusterName *string, namespace *string, version *string) {
	message, err := GenerateDeployMessage(application, clusterName, namespace, version)
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
