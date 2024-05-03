package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime/metrics"
	"strings"
	"sync"
	"time"

	"github.com/Payback159/openfero/pkg/logger"
	"github.com/Payback159/openfero/pkg/metadata"
	"github.com/ghodss/yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

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

var alertStore []Alert

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func initKubeClient() *kubernetes.Clientset {
	var kubeconfig *string

	// prepare kubernetes client with in cluster configuration
	var config *rest.Config
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		logger.Debug("Using out of cluster configuration")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		logger.Debug("Using in cluster configuration")
	}
	flag.Parse()

	//use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logger.Fatal("Could not read k8s cluster configuration: %s", zap.String("error", err.Error()))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Could not create k8s client: %s", zap.String("error", err.Error()))
	}

	return clientset
}

func main() {

	// Parse command line arguments
	addr := flag.String("addr", ":8080", "address to listen for webhook")
	logLevel := flag.String("logLevel", "info", "log level")

	flag.Parse()

	// configure logger
	var cfg zap.Config
	switch strings.ToLower(*logLevel) {
	case "debug":
		cfg = zap.NewDevelopmentConfig()
	case "info":
		cfg = zap.NewProductionConfig()
	default:
		logger.Fatal("Invalid log level specified")
	}

	// activate json logging
	if logger.SetConfig(cfg) != nil {
		logger.Fatal("Could not set logger configuration")
	}

	// Use the in-cluster config to create a kubernetes client
	clientset := initKubeClient()
	defaultNamespaceLocation := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	currentNamespace := ""

	//Check if running in-cluster or out-of-cluster
	_, err := rest.InClusterConfig()
	if err != nil {
		logger.Debug("Using out of cluster configuration")
		// Extract the current namespace from the client config
		currentNamespace, _, err = clientcmd.DefaultClientConfig.Namespace()
		if err != nil {
			logger.Fatal("Current kubernetes namespace could not be found", zap.String("error", err.Error()))
		}
	} else {
		logger.Debug("Using in cluster configuration")
		// Extract the current namespace from the mounted secrets
		if _, err := os.Stat(defaultNamespaceLocation); os.IsNotExist(err) {
			logger.Fatal("Current kubernetes namespace could not be found", zap.String("error", err.Error()))
		}
		namespaceDat, err := os.ReadFile(defaultNamespaceLocation)
		if err != nil {
			logger.Fatal("Couldn't read from "+defaultNamespaceLocation, zap.String("error", err.Error()))
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

	//register metrics and set prometheus handler
	addMetricsToPrometheusRegistry()
	http.Handle(metadata.MetricsPath, promhttp.Handler())

	logger.Info("Starting webhook receiver")
	http.HandleFunc("GET /healthz", server.healthzGetHandler)
	http.HandleFunc("GET /readiness", server.readinessGetHandler)
	http.HandleFunc("GET /alertStore", server.alertStoreGetHandler)
	http.HandleFunc("GET /alerts", server.alertsGetHandler)
	http.HandleFunc("POST /alerts", server.alertsPostHandler)
	http.HandleFunc("GET /ui", uiHandler)
	http.HandleFunc("GET /assets/", assetsHandler)

	logger.Info("Starting server on " + *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		logger.Fatal("error starting server: ", zap.String("error", err.Error()))
	}
}

// function which registers metrics to the prometheus registry
func addMetricsToPrometheusRegistry() {
	// Get descriptions for all supported metrics.
	metricsMeta := metrics.All()
	// Register metrics and retrieve the values in prometheus client
	for i := range metricsMeta {
		meta := metricsMeta[i]
		opts := getMetricsOptions(metricsMeta[i])
		if meta.Cumulative {
			// Register as a counter
			funcCounter := prometheus.NewCounterFunc(prometheus.CounterOpts(opts), func() float64 {
				return metadata.GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcCounter)
		} else {
			// Register as a gauge
			funcGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
				return metadata.GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcGauge)
		}
	}
}

// getMetricsOptions function to get prometheus options for a metric
func getMetricsOptions(metric metrics.Description) prometheus.Opts {
	tokens := strings.Split(metric.Name, "/")
	fmt.Printf("%s\n", metric.Name)
	if len(tokens) < 2 {
		return prometheus.Opts{}
	}
	nameTokens := strings.Split(tokens[len(tokens)-1], ":")
	// create a unique name for metric, that will be its primary key on the registry
	validName := normalizePrometheusName(strings.Join(nameTokens[:2], "_"))
	subsystem := metadata.GetMetricSubsystemName(metric)

	units := nameTokens[1]
	help := fmt.Sprintf("Units:%s, %s", units, metric.Description)
	opts := prometheus.Opts{
		Namespace: tokens[1],
		Subsystem: subsystem,
		Name:      validName,
		Help:      help,
	}
	return opts
}

// normalizePrometheusName function to normalize prometheus name
func normalizePrometheusName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "-", "_"))
}

// Use crypto/rand to generate a random string of a given length and charset
func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			logger.Error("error generating random string: ", zap.String("error", err.Error()))
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
		logger.Error("error listing ConfigMaps: ", zap.String("error", err.Error()))
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
		logger.Error("error encoding messages: ", zap.String("error", err.Error()))
		http.Error(httpwriter, "error encoding messages", http.StatusInternalServerError)
	}
}

// Handling the Alertmanager Post-Requests
func (server *clientsetStruct) alertsPostHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {

	dec := json.NewDecoder(httprequest.Body)
	defer httprequest.Body.Close()

	var message HookMessage
	if err := dec.Decode(&message); err != nil {
		logger.Error("error decoding message: ", zap.String("error", err.Error()))
		http.Error(httpwriter, "invalid request body", http.StatusBadRequest)
		return
	}

	status := sanitizeInput(message.Status)
	alertcount := len(message.Alerts)
	var waitgroup sync.WaitGroup
	waitgroup.Add(alertcount)

	logger.Info(status + " webhook received with " + fmt.Sprint(alertcount) + " alerts")

	if status == "resolved" || status == "firing" {
		logger.Info("Create ResponseJobs")
		for _, alert := range message.Alerts {
			go server.createResponseJob(&waitgroup, alert, status, httpwriter)
		}
		waitgroup.Wait()
	} else {
		logger.Warn("Status of alert was neither firing nor resolved, stop creating a response job.")
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
	logger.Info("Try to load configmap " + responsesConfigmap)
	configMap, err := server.clientset.CoreV1().ConfigMaps(server.configmapNamespace).Get(context.TODO(), responsesConfigmap, metav1.GetOptions{})
	if err != nil {
		logger.Error("error getting configmap: ", zap.String("error", err.Error()))
		return
	}

	jobDefinition := configMap.Data[alertname]
	var yamlJobDefinition []byte
	if jobDefinition != "" {
		yamlJobDefinition = []byte(jobDefinition)
	} else {
		logger.Error("Could not find a data block with the key " + alertname + " in the configmap.")
		return
	}
	// yamlJobDefinition contains a []byte of the yaml job spec
	// convert the yaml to json so it works with Unmarshal
	jsonBytes, err := yaml.YAMLToJSON(yamlJobDefinition)
	if err != nil {
		logger.Error("error while converting YAML job definition to JSON: ", zap.String("error", err.Error()))
		return
	}
	randomstring := StringWithCharset(5, charset)

	jobObject := &batchv1.Job{}
	err = json.Unmarshal(jsonBytes, jobObject)
	if err != nil {
		logger.Error("Error while using unmarshal on received job: ", zap.String("error", err.Error()))
		return
	}

	// Adding randomString to avoid name conflict
	jobObject.SetName(jobObject.Name + "-" + randomstring)
	// Adding Labels as Environment variables
	logger.Info("Adding Alert-Labels as environment variable to job " + jobObject.Name)
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
	logger.Info("Creating job " + jobObject.Name)
	_, err = jobsClient.Create(context.TODO(), jobObject, metav1.CreateOptions{})
	if err != nil {
		logger.Error("error creating job: ", zap.String("error", err.Error()))
		return
	}
	logger.Info("Created job " + jobObject.Name)
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
	if len(alertStore) >= alertsArraySize {
		alertStore = alertStore[1:]
	}
	alertStore = append(alertStore, alert)
}

// function which filters alerts based on the query
func filterAlerts(alerts []Alert, query string) []Alert {
	var filteredAlerts []Alert
	for _, alert := range alerts {
		matches := false

		// Check alertname
		if strings.Contains(strings.ToLower(alert.Labels["alertname"]), strings.ToLower(query)) {
			matches = true
		}

		// Check labels (case-insensitive)
		for _, value := range alert.Labels {
			if strings.Contains(strings.ToLower(value), strings.ToLower(query)) {
				matches = true
				break
			}
		}

		// Check annotations (case-insensitive)
		for _, value := range alert.Annotations {
			if strings.Contains(strings.ToLower(value), strings.ToLower(query)) {
				matches = true
				break
			}
		}

		if matches {
			filteredAlerts = append(filteredAlerts, alert)
		}
	}
	return filteredAlerts
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Called asset " + r.URL.Path)
	// set content type based on file extension
	contentType := ""
	switch filepath.Ext(r.URL.Path) {
	case ".css":
		contentType = "text/css"
	case ".js":
		contentType = "application/javascript"
	}
	w.Header().Set("Content-Type", contentType)

	// sanitize the URL path to prevent path traversal
	path, err := verifyPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid path specified", http.StatusBadRequest)
		return
	}

	logger.Debug("Called asset " + r.URL.Path + " serves Filesystem asset: " + path)
	// serve assets from the web/assets directory
	http.ServeFile(w, r, path)
}

// verifyPath verifies and evaluates the given path to ensure it is safe and valid.
// It checks if the path is within the trusted root directory and evaluates any symbolic links.
// If the path is unsafe or invalid, it returns an error.
// Otherwise, it returns the evaluated path.
func verifyPath(path string) (string, error) {
	//TODO: move wd out of function to initialize once
	errmsg := "unsafe or invalid path specified"
	wd, err := os.Getwd()
	if err != nil {
		logger.Error("Error getting working directory: ", zap.String("error", err.Error()))
		return path, errors.New(errmsg)
	}
	trustedRoot := wd + "/web"
	logger.Debug("Trusted root directory: " + trustedRoot)

	logger.Debug("Verifying path " + path)
	p, err := filepath.EvalSymlinks(trustedRoot + path)
	if err != nil {
		fmt.Println("Error " + err.Error())
		return path, errors.New(errmsg)
	}

	logger.Debug("Evaluated path " + p)
	err = inTrustedRoot(p, trustedRoot)
	if err != nil {
		fmt.Println("Error " + err.Error())
		return p, errors.New(errmsg)
	} else {
		return p, nil
	}
}

// inTrustedRoot checks if a given path is within the trusted root directory.
// It iteratively goes up the directory tree until it reaches the root directory ("/")
// or the trusted root directory. If the path is within the trusted root directory,
// it returns nil. Otherwise, it returns an error indicating that the path is outside
// of the trusted root.
func inTrustedRoot(path string, trustedRoot string) error {
	for path != "/" {
		path = filepath.Dir(path)
		if path == trustedRoot {
			return nil
		}
	}
	return errors.New("path is outside of trusted root")
}

// function which provides alerts array to the getHandler
func (server *clientsetStruct) alertStoreGetHandler(w http.ResponseWriter, r *http.Request) {
	var alerts []Alert
	// Get search query parameter
	query := r.URL.Query().Get("q")

	// Filter alerts if query is provided
	if query != "" {
		alerts = filterAlerts(alertStore, query)
	} else {
		alerts = alertStore
	}

	w.Header().Set(CONTENTTYPE, APPLICATIONJSON)
	err := json.NewEncoder(w).Encode(alerts)
	if err != nil {
		logger.Error("error encoding alerts: ", zap.String("error", err.Error()))
		http.Error(w, "error encoding alerts", http.StatusInternalServerError)
	}
}

// function which provides the UI to the user
func uiHandler(w http.ResponseWriter, r *http.Request) {
	var alerts []Alert
	w.Header().Set(CONTENTTYPE, "text/html")
	//Parse the templates in web/templates/
	tmpl, err := template.ParseFiles("web/templates/alertStore.html.templ")
	if err != nil {
		logger.Error("error parsing templates: ", zap.String("error", err.Error()))
		http.Error(w, "error parsing templates", http.StatusInternalServerError)
	}

	query := r.URL.Query().Get("q")

	alerts = getAlerts(query)

	s := struct {
		Title  string
		Alerts []Alert
	}{
		Title:  "Alert Store",
		Alerts: alerts,
	}

	//Execute the templates
	err = tmpl.Execute(w, s)
	if err != nil {
		logger.Error("error executing templates: ", zap.String("error", err.Error()))
		http.Error(w, "error executing templates", http.StatusInternalServerError)
	}
}

// function which gets alerts from the alertStore
func getAlerts(query string) []Alert {
	resp, err := http.Get("http://localhost:8080/alertStore?q=" + query)
	if err != nil {
		logger.Error("error getting alerts: ", zap.String("error", err.Error()))
	}
	defer resp.Body.Close()
	var alerts []Alert
	err = json.NewDecoder(resp.Body).Decode(&alerts)
	if err != nil {
		logger.Error("error decoding alerts: ", zap.String("error", err.Error()))
	}
	return alerts
}
