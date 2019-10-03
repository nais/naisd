package main

import (
	"flag"
	"github.com/nais/naisd/pkg/event"
	"github.com/nais/naisd/pkg/kafka"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	"github.com/nais/naisd/api"
)

const Port = ":8081"

var kafkaConfig = kafka.Config{}

var kafkaBrokers string

func init() {
	defaultGroup := kafka.DefaultGroupName()

	flag.StringVar(&kafkaBrokers, "kafka.brokers", "localhost:9092", "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&kafkaConfig.Topic, "kafka.topic", "deploymentEvents", "Kafka topic for deployment status.")
	flag.StringVar(&kafkaConfig.ClientID, "kafka.client-id", defaultGroup, "Kafka client ID.")
	flag.StringVar(&kafkaConfig.GroupID, "kafka.group-id", defaultGroup, "Kafka consumer group ID.")
	flag.BoolVar(&kafkaConfig.SASL.Enabled, "kafka.sasl.enabled", false, "Enable SASL authentication.")
	flag.BoolVar(&kafkaConfig.SASL.Handshake, "kafka.sasl.handshake", true, "Use handshake for SASL authentication.")
	flag.StringVar(&kafkaConfig.SASL.Username, "kafka.sasl.username", os.Getenv("KAFKA_SASL_USERNAME"), "Username for Kafka authentication.")
	flag.StringVar(&kafkaConfig.SASL.Password, "kafka.sasl.password", os.Getenv("KAFKA_SASL_PASSWORD"), "Password for Kafka authentication.")
	flag.BoolVar(&kafkaConfig.TLS.Enabled, "kafka.tls.enabled", false, "Use TLS for connecting to Kafka.")
	flag.BoolVar(&kafkaConfig.TLS.Insecure, "kafka.tls.insecure", false, "Allow insecure Kafka TLS connections.")
	flag.BoolVar(&kafkaConfig.Enabled, "kafka.enabled", false, "Enable connection to kafka")
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	fasitUrl := flag.String("fasit-url", "https://fasit.example.no", "URL to fasit instance")
	clusterSubdomain := flag.String("cluster-subdomain", "nais-example.nais.example.no", "Cluster sub-domain")
	clusterName := flag.String("clustername", "kubernetes", "Name of the kubernetes cluster")
	istioEnabled := flag.Bool("istio-enabled", false, "If istio is enabled or not")
	authenticationEnabled := flag.Bool("authentication-enabled", false, "If authentication is enabled or not")

	flag.Parse()

	glog.Infof("using fasit instance %s", *fasitUrl)
	glog.Infof("running on port %s", Port)
	glog.Infof("istio enabled = %t", *istioEnabled)
	glog.Infof("authentication enabled = %t", *authenticationEnabled)
	glog.Infof("kafka enabled = %t", kafkaConfig.Enabled)

	deploymentEventHandler := func(event deployment.Event) {}

	if kafkaConfig.Enabled {
		kafkaConfig.Brokers = strings.Split(kafkaBrokers, ",")
		glog.Infof("kafka brokers = %v", kafkaConfig.Brokers)
		glog.Infof("kafka topic = %s", kafkaConfig.Topic)
		glog.Infof("kafka tls enabled = %t", kafkaConfig.TLS.Enabled)
		if kafkaConfig.SASL.Enabled {
			glog.Infof("kafka username = %s", kafkaConfig.SASL.Username)
		}
		kafkaLogger := log.New(os.Stdout, "kafka] ", log.LstdFlags)
		sarama.Logger = kafkaLogger
		kafkaClient, err := kafka.NewClient(&kafkaConfig)
		if err != nil {
			log.Fatalf("unable to setup kafka: %s", err)
		}
		go kafkaClient.ProducerLoop()
		deploymentEventHandler = kafkaClient.Send
	}

	clientSet := newClientSet(*kubeconfig)
	deploymentStatusViewer := api.NewDeploymentStatusViewer(clientSet)
	naisd := api.NewAPI(
		clientSet,
		*fasitUrl,
		*clusterSubdomain,
		*clusterName,
		*istioEnabled,
		*authenticationEnabled,
		deploymentStatusViewer,
		deploymentEventHandler,
	)
	err := http.ListenAndServe(Port, naisd.Handler())
	if err != nil {
		panic(err)
	}
}

// returns config using kubeconfig if provided, else from cluster context
func newClientSet(kubeconfig string) kubernetes.Interface {

	var config *rest.Config
	var err error

	if kubeconfig != "" {
		glog.Infof("using provided kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		glog.Infof("no kubeconfig provided, assuming we are running inside a cluster")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	return clientset
}
