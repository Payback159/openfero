package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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
	//activate json logging
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("Starting webhook receiver")

	// Extract the current namespace from the mounted secrets
	default_k8s_namespace_location := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if _, err := os.Stat(default_k8s_namespace_location); os.IsNotExist(err) {
		log.Panic("Current kubernetes namespace could not be found", err.Error())
	}

	namespace_dat, err := ioutil.ReadFile(default_k8s_namespace_location)
	if err != nil {
		log.Panic("Couldn't read from "+default_k8s_namespace_location, err.Error())
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

	http.HandleFunc("/healthz", server.healthzHandler)
	http.HandleFunc("/readiness", server.readinessHandler)
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
func (server *clientsetStruct) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	log.Info("Health request answered")
}

//handling readiness probe
func (server *clientsetStruct) readinessHandler(w http.ResponseWriter, r *http.Request) {
	_, err := server.clientset.CoreV1().ConfigMaps(server.configmap_namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
		http.Error(w, "ConfigMaps could not be listed. Does the ServiceAccount of OpenFero also have the necessary permissions?", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	log.Info("Readiness request answered")
}

func (server *clientsetStruct) alertsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		server.getHandler(w, r)
	case http.MethodPost:
		server.postHandler(w, r)
	default:
		http.Error(w, "unsupported HTTP method", http.StatusBadRequest)
	}
}

//Handling get requests to listen received alerts
func (server *clientsetStruct) getHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {
	//Alertmanager expects an 200 OK response, otherwise send_resolved will never work
	enc := json.NewEncoder(httpwriter)
	httpwriter.Header().Set("Content-Type", "application/json")
	httpwriter.WriteHeader(http.StatusOK)

	if err := enc.Encode("OK"); err != nil {
		log.Error("error encoding messages: ", err)
	}
}

//Handling the Alertmanager Post-Requests
func (server *clientsetStruct) postHandler(httpwriter http.ResponseWriter, httprequest *http.Request) {

	dec := json.NewDecoder(httprequest.Body)
	defer httprequest.Body.Close()

	var message HookMessage
	if err := dec.Decode(&message); err != nil {
		log.Error("error decoding message: ", err)
		http.Error(httpwriter, "invalid request body", http.StatusBadRequest)
		return
	}

	status := sanitize_input(message.Status)
	alertcount := len(message.Alerts)

	log.Info(status + " webhook received with " + fmt.Sprint(alertcount) + " alerts")

	if status == "resolved" || status == "firing" {
		log.Info("Create ResponseJobs")
		server.createResponseJob(message, status, httpwriter)
	} else {
		log.Warn("Status of alert was neither firing nor resolved, stop creating a response job.")
		return
	}

}

func sanitize_input(input string) string {
	input = strings.Replace(input, "\n", "", -1)
	input = strings.Replace(input, "\r", "", -1)
	return input
}

func (server *clientsetStruct) createResponseJob(message HookMessage, status string, httpwriter http.ResponseWriter) {
	for _, alert := range message.Alerts {
		alertname := sanitize_input(alert.Labels["alertname"])
		responses_configmap := strings.ToLower("openfero-" + alertname + "-" + status)
		log.Info("Try to load configmap " + responses_configmap)
		configMap, err := server.clientset.CoreV1().ConfigMaps(server.configmap_namespace).Get(context.TODO(), responses_configmap, metav1.GetOptions{})
		if err != nil {
			log.Error(err)
			continue
		}

		job_definition := configMap.Data[alertname]
		var yaml_job_definition []byte
		if job_definition != "" {
			yaml_job_definition = []byte(job_definition)
		} else {
			log.Error("Could not find a data block with the key " + alertname + " in the configmap.")
			continue
		}
		// yaml_job_definition contains a []byte of the yaml job spec
		// convert the yaml to json so it works with Unmarshal
		jsonBytes, err := yaml.YAMLToJSON(yaml_job_definition)
		if err != nil {
			log.Error("error while converting YAML job definition to JSON: ", err)
			continue
		}
		randomstring := StringWithCharset(5, charset)

		jobObject := &batchv1.Job{}
		err = json.Unmarshal(jsonBytes, jobObject)
		if err != nil {
			log.Error("Error while using unmarshal on received job: ", err)
			continue
		}

		//Adding randomString to avoid name conflict
		jobObject.SetName(jobObject.Name + "-" + randomstring)
		//Adding Labels as Environment variables
		log.Info("Adding Alert-Labels as environment variable to job " + jobObject.Name)
		for labelkey, labelvalue := range alert.Labels {
			jobObject.Spec.Template.Spec.Containers[0].Env = append(jobObject.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{Name: "OPENFERO_" + strings.ToUpper(labelkey), Value: labelvalue})
		}

		// Job client for creating the job according to the job definitions extracted from the responses configMap
		jobsClient := server.clientset.BatchV1().Jobs(server.job_destination_namespace)

		// Create job
		log.Info("Creating job " + jobObject.Name)
		_, err = jobsClient.Create(context.TODO(), jobObject, metav1.CreateOptions{})
		if err != nil {
			log.Error("error creating job: ", err)
			continue
		}
		log.Info("Created job " + jobObject.Name)
	}
}

func (server *clientsetStruct) cleanupJobs() {
	jobClient := server.clientset.BatchV1().Jobs(server.job_destination_namespace)
	deletepropagationpolicy := metav1.DeletePropagationBackground
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &deletepropagationpolicy}

	jobs, _ := jobClient.List(context.TODO(), metav1.ListOptions{})

	for _, job := range jobs.Items {
		if job.Status.Active > 0 {
			log.Info("Job " + job.Name + " is running")
		} else {
			if job.Status.Succeeded > 0 {
				log.Info("Job " + job.Name + " succeeded - going to cleanup")
				jobClient.Delete(context.TODO(), job.Name, deleteOptions)
			}
		}
	}

}
