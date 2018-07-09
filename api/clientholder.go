package api

import (
	"k8s.io/client-go/kubernetes"
)

type clientHolder struct {
	client kubernetes.Interface
}
