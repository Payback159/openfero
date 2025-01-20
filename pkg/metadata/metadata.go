package metadata

import (
	"fmt"

	metrics "runtime/metrics"
	"strings"

	log "github.com/Payback159/openfero/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	errorValue          = -1.0
	OtelScope           = "https://github.com/Payback159/openfero"
	MetricsPath         = "/metrics"
	MetricsEndpointPort = ":2223"
)

var (
	JobsCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{

		Name: "openfero_jobs_created_total",

		Help: "Total number of jobs created",
	})

	JobsSucceededTotal = prometheus.NewCounter(prometheus.CounterOpts{

		Name: "openfero_jobs_succeeded_total",

		Help: "Total number of jobs succeeded",
	})

	JobsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{

		Name: "openfero_jobs_failed_total",

		Help: "Total number of jobs failed",
	})
)

// Function to get metrics values from runtime/metrics package as float64
func GetSingleMetricFloat(metricName string) float64 {

	// Create a sample for the metric.
	sample := make([]metrics.Sample, 1)
	sample[0].Name = metricName

	// Sample the metric.
	metrics.Read(sample)

	return getFloat64(sample[0])
}

// function to return differemt sample values as float 64
// curently it handles single values, in future it will handle histograms
func getFloat64(sample metrics.Sample) float64 {
	var floatVal float64
	// Handle each sample.
	switch sample.Value.Kind() {
	case metrics.KindUint64:
		floatVal = float64(sample.Value.Uint64())
	case metrics.KindFloat64:
		floatVal = float64(sample.Value.Float64())
	case metrics.KindBad:
		log.Error("bug in runtime/metrics package!")
	case metrics.KindFloat64Histogram:
		// Handle histogram kind if needed
		floatVal = 0.0 // Placeholder, update with actual logic if needed
	default:
		log.Error(fmt.Sprintf("%s: unexpected metric Kind: %v", sample.Name, sample.Value.Kind()))
	}
	return floatVal
}

// Function to get metrics subsysyetm from a mteric metadata
func getMetricSubsystemName(metric metrics.Description) string {
	tokens := strings.Split(metric.Name, "/")
	if len(tokens) < 2 {
		return ""
	}
	if len(tokens) > 3 {
		subsystemTokens := tokens[2 : len(tokens)-1]
		subsystem := strings.Join(subsystemTokens, "_")
		subsystem = strings.ReplaceAll(subsystem, "-", "_")
		return subsystem
	}
	return ""
}

// function which registers metrics to the prometheus registry
func AddMetricsToPrometheusRegistry() {
	// Register the metrics with Prometheus
	prometheus.MustRegister(JobsCreatedTotal)
	prometheus.MustRegister(JobsSucceededTotal)
	prometheus.MustRegister(JobsFailedTotal)
	// Get descriptions for all supported metrics.
	metricsMeta := metrics.All()
	// Register metrics and retrieve the values in prometheus client
	for i := range metricsMeta {
		meta := metricsMeta[i]
		opts := getMetricsOptions(metricsMeta[i])
		if meta.Cumulative {
			// Register as a counter
			funcCounter := prometheus.NewCounterFunc(prometheus.CounterOpts(opts), func() float64 {
				return GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcCounter)
		} else {
			// Register as a gauge
			funcGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
				return GetSingleMetricFloat(meta.Name)
			})
			prometheus.MustRegister(funcGauge)
		}
	}
}

// getMetricsOptions function to get prometheus options for a metric
func getMetricsOptions(metric metrics.Description) prometheus.Opts {
	tokens := strings.Split(metric.Name, "/")
	if len(tokens) < 2 {
		log.Error("error getting metric options: invalid metric name", zap.String("metric", metric.Name))
		return prometheus.Opts{}
	}
	nameTokens := strings.Split(tokens[len(tokens)-1], ":")
	// create a unique name for metric, that will be its primary key on the registry
	validName := normalizePrometheusName(strings.Join(nameTokens[:2], "_"))
	subsystem := getMetricSubsystemName(metric)

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
