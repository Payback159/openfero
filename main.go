package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/go-co-op/gocron"
)

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
		clientset                 kubernetes.Clientset
		job_destination_namespace string
		configmap_namespace       string
	}
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()),
)

func main() {
	log.Printf("Starting webhook receiver")

	// Extract the current namespace from the mounted secrets
	default_k8s_namespace_location := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if _, err := os.Stat(default_k8s_namespace_location); os.IsNotExist(err) {
		log.Printf("Current kubernetes namespace could not be found")
		panic(err.Error())
	}

	namespace_dat, err := ioutil.ReadFile(default_k8s_namespace_location)
	if err != nil {
		log.Printf("Couldn't read from %s", default_k8s_namespace_location)
		panic(err.Error())
	}

	current_namespace := string(namespace_dat)

	configmap_namespace := flag.String("configmap_namespace", current_namespace, "Kubernetes namespace where jobs are defined")
	job_destination_namespace := flag.String("job_destination_namespace", current_namespace, "Kubernetes namespace where jobs will be created")

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	server := &clientsetStruct{
		clientset:                 *clientset,
		job_destination_namespace: *job_destination_namespace,
		configmap_namespace:       *configmap_namespace,
	}

	addr := flag.String("addr", ":8080", "address to listen for webhook")
	flag.Parse()

	scheduler := gocron.NewScheduler(time.UTC)
	cleanupjob, _ := scheduler.Every("5m").Do(server.cleanupJobs)
	cleanupjob.SingletonMode()
	scheduler.StartAsync()

	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readiness", readinessHandler)
	http.HandleFunc("/alerts", server.alertsHandler)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func StringWithCharset(length int, charset string) string {

	randombytes := make([]byte, length)
	for i := range randombytes {
		randombytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randombytes)
}

//handling healthness probe
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "ok\n")
}

//handling readiness probe
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "ok\n")
}

func (server *clientsetStruct) alertsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		server.getHandler(w, r)
	case http.MethodPost:
		server.postHandler(w, r)
	default:
		http.Error(w, "unsupported HTTP method", 400)
	}
}

//Handling get requests to listen received alerts
func (server *clientsetStruct) getHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {
	//Alertmanager expects an 200 OK response, otherwise send_resolved will never work
	enc := json.NewEncoder(httpwriter)
	httpwriter.Header().Set("Content-Type", "application/json")
	httpwriter.WriteHeader(http.StatusOK)

	if err := enc.Encode("OK"); err != nil {
		log.Printf("error encoding messages: %v", err)
	}
}

//Handling the Alertmanager Post-Requests
func (server *clientsetStruct) postHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {

	dec := json.NewDecoder(httprequest.Body)
	defer httprequest.Body.Close()

	var message HookMessage
	if err := dec.Decode(&message); err != nil {
		log.Printf("error decoding message: %v", err)
		http.Error(httpwriter, "invalid request body", 400)
		return
	}

	status := message.Status
	alertname := message.CommonLabels["alertname"]
	alertcount := len(message.Alerts)

	log.Printf("Webhook received: " + alertname + "[" + status + "] with " + fmt.Sprint(alertcount) + " Alerts")

	for labelkey, labelvalue := range message.CommonLabels {
		log.Printf("Label key[%s] value[%s]\n", labelkey, labelvalue)
	}
	for annotationkey, annotationvalue := range message.CommonAnnotations {
		log.Printf("Annotation key[%s] value[%s]\n", annotationkey, annotationvalue)
	}

	if status == "resolved" {
		server.createResponseJob(message, status, httpwriter)
	} else if status == "firing" {
		server.createResponseJob(message, status, httpwriter)
	} else {
		log.Printf("Received alarm without correct response configuration, ommiting reponses.")
		return
	}

	fmt.Println(message.CommonLabels["namespace"])
}

func (server *clientsetStruct) createResponseJob(message HookMessage, status string, httpwriter http.ResponseWriter) {

	alertname := message.CommonLabels["alertname"]
	responses_configmap := strings.ToLower("openfero-" + alertname + "-" + status)
	configMap, err := server.clientset.CoreV1().ConfigMaps(server.configmap_namespace).Get(responses_configmap, metav1.GetOptions{})
	if err != nil {
		log.Printf("error while retrieving the configMap: "+responses_configmap, err)
		http.Error(httpwriter, "Webhook error retrieving configMap with job definitions", 500)
		return
	}

	if configMap.Labels["openfero/job-disabled"] != "" {
		log.Printf("Found openfero/job-disabled label in job-configmap %s", configMap.Name)
		return
	}

	job_definition := configMap.Data[alertname]
	yaml_job_definition := []byte(job_definition)

	// yaml_job_definition contains a []byte of the yaml job spec
	// convert the yaml to json so it works with Unmarshal
	jsonBytes, err := yaml.YAMLToJSON(yaml_job_definition)
	if err != nil {
		log.Printf("error while converting YAML job definition to JSON: %v", err)
		http.Error(httpwriter, "Webhook error creating a job", 500)
		return
	}

	for alert := range message.Alerts {
		randomstring := StringWithCharset(5, charset)

		jobObject := &batchv1.Job{}
		err = json.Unmarshal(jsonBytes, jobObject)
		if err != nil {
			log.Printf("Error while using unmarshal on received job: %v", err)
			http.Error(httpwriter, "Webhook error creating a job", 500)
			return
		}

		//Adding randomString to avoid name conflict
		jobObject.SetName(jobObject.Name + "-" + randomstring)
		//Adding Labels as Environment variables
		for labelkey, labelvalue := range message.Alerts[alert].Labels {
			jobObject.Spec.Template.Spec.Containers[0].Env = append(jobObject.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{Name: "OPENFERO_" + strings.ToUpper(labelkey), Value: labelvalue})
		}

		// Job client for creating the job according to the job definitions extracted from the responses configMap
		jobsClient := server.clientset.BatchV1().Jobs(server.job_destination_namespace)

		// Create job
		log.Printf("Creating job... ")
		result, err := jobsClient.Create(jobObject)
		if err != nil {
			log.Printf("error creating job: %v", err)
			http.Error(httpwriter, "Webhook error creating a job", 500)
			return
		}
		log.Printf("Created job -->\n %q.", result)
	}
}

func (server *clientsetStruct) cleanupJobs() {
	jobClient := server.clientset.BatchV1().Jobs(server.job_destination_namespace)
	deletepropagationpolicy := metav1.DeletePropagationBackground
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &deletepropagationpolicy}

	jobs, _ := jobClient.List(metav1.ListOptions{})

	for _, job := range jobs.Items {
		if job.Status.Active > 0 {
			log.Printf("Job %v is running", job.Name)
		} else {
			if job.Status.Succeeded > 0 {
				log.Printf("Job %v succeeded... going to cleanup", job.Name)
				jobClient.Delete(job.Name, &deleteOptions)
			}
		}
	}

}
