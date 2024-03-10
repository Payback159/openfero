package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ghodss/yaml"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const CONTENTTYPE = "Content-Type"
const APPLICATIONJSON = "application/json"

type (
	Timestamp   time.Time
	HookMessage struct {
		Version           string            `json:"version"`
		GroupKey          string            `json:"groupKey"`
		Status            string            `json:"status"`
		Receiver          string            `json:"receiver"`
		GroupLabels       map[string]string `json:"groupLabels"`
		CommonLabels      map[string]string `json:"commonLabels"`
		CommonAnnotations map[string]string `json:"commonAnnotations"`
		ExternalURL       string            `json:"externalURL"`
		Alerts            []Alert           `json:"alerts"`
	}

	Alert struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    string            `json:"startsAt,omitempty"`
		EndsAt      string            `json:"EndsAt,omitempty"`
	}

	clientsetStruct struct {
		clientset               kubernetes.Clientset
		jobDestinationNamespace string
		configmapNamespace      string
	}
)

var alerts []Alert
var log *zap.Logger

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func initKubeClient() *kubernetes.Clientset {
	var kubeconfig *string

	// prepare kubernetes client with in cluster configuration
	var config *rest.Config
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		log.Debug("Using out of cluster configuration")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		log.Debug("Using in cluster configuration")
	}
	flag.Parse()

	//use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(fmt.Sprintf("Could not read k8s cluster configuration: %s", err))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not create k8s client: %s", err))
	}

	return clientset
}

func main() {

	// Parse command line arguments
	addr := flag.String("addr", ":8080", "address to listen for webhook")
	logLevel := flag.String("logLevel", "info", "log level")
	flag.Parse()

	// Konfiguriere das Log-Level
	var cfg zap.Config
	switch strings.ToLower(*logLevel) {
	case "debug":
		cfg = zap.NewDevelopmentConfig()
	case "info":
		cfg = zap.NewProductionConfig()
	default:
		log.Fatal("Invalid log level specified")
	}

	// Aktiviere das JSON-Logging
	logger, err := cfg.Build()
	if err != nil {
		log.Fatal("Error initializing the logger: ", zap.String("error", err.Error()))
	}
	defer func() {
		syncErr := logger.Sync()
		if syncErr != nil {
			log.Fatal("Error syncing the logger: ", zap.String("error", syncErr.Error()))
		}
	}()
	log = logger
	log.Info("Starting webhook receiver")

	// Use the in-cluster config to create a kubernetes client
	clientset := initKubeClient()
	defaultNamespaceLocation := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	currentNamespace := ""

	//Check if running in-cluster or out-of-cluster
	_, err = rest.InClusterConfig()
	if err != nil {
		log.Debug("Using out of cluster configuration")
		// Extract the current namespace from the client config
		currentNamespace, _, err = clientcmd.DefaultClientConfig.Namespace()
		if err != nil {
			log.Fatal("Current kubernetes namespace could not be found", zap.String("error", err.Error()))
		}
	} else {
		log.Debug("Using in cluster configuration")
		// Extract the current namespace from the mounted secrets
		if _, err := os.Stat(defaultNamespaceLocation); os.IsNotExist(err) {
			log.Fatal("Current kubernetes namespace could not be found", zap.String("error", err.Error()))
		}
		namespaceDat, err := os.ReadFile(defaultNamespaceLocation)
		if err != nil {
			log.Fatal("Couldn't read from "+defaultNamespaceLocation, zap.String("error", err.Error()))
		}
		currentNamespace = string(namespaceDat)
	}

	configmapNamespace := flag.String("configmapNamespace", currentNamespace, "Kubernetes namespace where jobs are defined")
	jobDestinationNamespace := flag.String("jobDestinationNamespace", currentNamespace, "Kubernetes namespace where jobs will be created")

	server := &clientsetStruct{
		clientset:               *clientset,
		jobDestinationNamespace: *jobDestinationNamespace,
		configmapNamespace:      *configmapNamespace,
	}

	http.HandleFunc("GET /healthz", server.healthzGetHandler)
	http.HandleFunc("GET /readiness", server.readinessGetHandler)
	http.HandleFunc("GET /alert-store", server.alertStoreGetHandler)
	http.HandleFunc("GET /alerts", server.alertsGetHandler)
	http.HandleFunc("POST /alerts", server.alertsPostHandler)

	log.Info("Starting server on " + *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("error starting server: ", zap.String("error", err.Error()))
	}
}

// Use crypto/rand to generate a random string of a given length and charset
func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			log.Error("error generating random string: ", zap.String("error", err.Error()))
		}
		randombytes[i] = charset[num.Int64()]
	}

	return string(randombytes)
}

// handling healthness probe
func (server *clientsetStruct) healthzGetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(CONTENTTYPE, APPLICATIONJSON)
	w.WriteHeader(http.StatusOK)
}

// handling readiness probe
func (server *clientsetStruct) readinessGetHandler(w http.ResponseWriter, r *http.Request) {
	_, err := server.clientset.CoreV1().ConfigMaps(server.configmapNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("error listing ConfigMaps: ", zap.String("error", err.Error()))
		http.Error(w, "ConfigMaps could not be listed. Does the ServiceAccount of OpenFero also have the necessary permissions?", http.StatusInternalServerError)
		return
	}
	w.Header().Set(CONTENTTYPE, APPLICATIONJSON)
	w.WriteHeader(http.StatusOK)
}

// Handling get requests to listen received alerts
func (server *clientsetStruct) alertsGetHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {
	// Alertmanager expects an 200 OK response, otherwise send_resolved will never work
	enc := json.NewEncoder(httpwriter)
	httpwriter.Header().Set(CONTENTTYPE, APPLICATIONJSON)
	httpwriter.WriteHeader(http.StatusOK)

	if err := enc.Encode("OK"); err != nil {
		log.Error("error encoding messages: ", zap.String("error", err.Error()))
		http.Error(httpwriter, "error encoding messages", http.StatusInternalServerError)
	}
}

// Handling the Alertmanager Post-Requests
func (server *clientsetStruct) alertsPostHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {

	dec := json.NewDecoder(httprequest.Body)
	defer httprequest.Body.Close()

	var message HookMessage
	if err := dec.Decode(&message); err != nil {
		log.Error("error decoding message: ", zap.String("error", err.Error()))
		http.Error(httpwriter, "invalid request body", http.StatusBadRequest)
		return
	}

	status := sanitizeInput(message.Status)
	alertcount := len(message.Alerts)
	var waitgroup sync.WaitGroup
	waitgroup.Add(alertcount)

	log.Info(status + " webhook received with " + fmt.Sprint(alertcount) + " alerts")

	if status == "resolved" || status == "firing" {
		log.Info("Create ResponseJobs")
		for _, alert := range message.Alerts {
			go server.createResponseJob(&waitgroup, alert, status, httpwriter)
		}
		waitgroup.Wait()
	} else {
		log.Warn("Status of alert was neither firing nor resolved, stop creating a response job.")
		return
	}

}

func sanitizeInput(input string) string {
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\r", "")
	return input
}

func (server *clientsetStruct) createResponseJob(waitgroup *sync.WaitGroup, alert Alert, status string, httpwriter http.ResponseWriter) {
	defer waitgroup.Done()
	server.saveAlert(alert)
	alertname := sanitizeInput(alert.Labels["alertname"])
	responsesConfigmap := strings.ToLower("openfero-" + alertname + "-" + status)
	log.Info("Try to load configmap " + responsesConfigmap)
	configMap, err := server.clientset.CoreV1().ConfigMaps(server.configmapNamespace).Get(context.TODO(), responsesConfigmap, metav1.GetOptions{})
	if err != nil {
		log.Error("error getting configmap: ", zap.String("error", err.Error()))
		return
	}

	jobDefinition := configMap.Data[alertname]
	var yamlJobDefinition []byte
	if jobDefinition != "" {
		yamlJobDefinition = []byte(jobDefinition)
	} else {
		log.Error("Could not find a data block with the key " + alertname + " in the configmap.")
		return
	}
	// yamlJobDefinition contains a []byte of the yaml job spec
	// convert the yaml to json so it works with Unmarshal
	jsonBytes, err := yaml.YAMLToJSON(yamlJobDefinition)
	if err != nil {
		log.Error("error while converting YAML job definition to JSON: ", zap.String("error", err.Error()))
		return
	}
	randomstring := StringWithCharset(5, charset)

	jobObject := &batchv1.Job{}
	err = json.Unmarshal(jsonBytes, jobObject)
	if err != nil {
		log.Error("Error while using unmarshal on received job: ", zap.String("error", err.Error()))
		return
	}

	// Adding randomString to avoid name conflict
	jobObject.SetName(jobObject.Name + "-" + randomstring)
	// Adding Labels as Environment variables
	log.Info("Adding Alert-Labels as environment variable to job " + jobObject.Name)
	for labelkey, labelvalue := range alert.Labels {
		jobObject.Spec.Template.Spec.Containers[0].Env = append(jobObject.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{Name: "OPENFERO_" + strings.ToUpper(labelkey), Value: labelvalue})
	}

	// Adding TTL to job if it is not already set
	if !checkJobTTL(jobObject) {
		addJobTTL(jobObject)
	}

	// Job client for creating the job according to the job definitions extracted from the responses configMap
	jobsClient := server.clientset.BatchV1().Jobs(server.jobDestinationNamespace)

	// Create job
	log.Info("Creating job " + jobObject.Name)
	_, err = jobsClient.Create(context.TODO(), jobObject, metav1.CreateOptions{})
	if err != nil {
		log.Error("error creating job: ", zap.String("error", err.Error()))
		return
	}
	log.Info("Created job " + jobObject.Name)
}

func checkJobTTL(jobObject *batchv1.Job) bool {
	return jobObject.Spec.TTLSecondsAfterFinished != nil
}

func addJobTTL(jobObject *batchv1.Job) {
	ttl := int32(300)
	jobObject.Spec.TTLSecondsAfterFinished = &ttl
}

// function which gets an alert from createResponseJob and saves it to the alerts array
// drops the oldest alert if the array is full
func (server *clientsetStruct) saveAlert(alert Alert) {
	alertsArraySize := 10
	if len(alerts) >= alertsArraySize {
		alerts = alerts[1:]
	}
	alerts = append(alerts, alert)
}

// function which provides alerts array to the getHandler
func (server *clientsetStruct) alertStoreGetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(CONTENTTYPE, APPLICATIONJSON)
	err := json.NewEncoder(w).Encode(alerts)
	if err != nil {
		log.Error("error encoding alerts: ", zap.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
