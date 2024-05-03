package metadata

import (
	"fmt"

	metrics "runtime/metrics"
	"strings"

	"github.com/Payback159/openfero/pkg/logger"
)

const (
	errorValue          = -1.0
	OtelScope           = "https://github.com/Payback159/openfero"
	MetricsPath         = "/metrics"
	MetricsEndpointPort = ":2223"
)

// Function to get metrics values from runtime/metrics package
func GetAllMetrics() []metrics.Sample {
	metricsMetadata := metrics.All()
	samples := make([]metrics.Sample, len(metricsMetadata))
	// update name of each sample
	for idx := range metricsMetadata {
		samples[idx].Name = metricsMetadata[idx].Name
	}
	metrics.Read(samples)
	return samples
}

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
	case metrics.KindFloat64Histogram:
		// TODO: implementation needed
		return errorValue
	case metrics.KindBad:
		logger.Error("bug in runtime/metrics package!")
	default:
		logger.Error(fmt.Sprintf("%s: unexpected metric Kind: %v\n", sample.Name, sample.Value.Kind()))
	}
	return floatVal
}

// Function to get metrics subsysyetm from a mteric metadata
func GetMetricSubsystemName(metric metrics.Description) string {
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
