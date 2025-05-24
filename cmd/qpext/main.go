package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	ioprometheusclient "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.uber.org/zap"
	"knative.dev/serving/pkg/queue/sharedmain"
)

var (
	EnvVars   = []string{"SERVING_SERVICE", "SERVING_CONFIGURATION", "SERVING_REVISION"}
	LabelKeys = []string{"service_name", "configuration_name", "revision_name"}
)

const (
	// aggregate scraping env vars from ome/pkg/constants
	ContainerPrometheusMetricsPortEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PORT"
	ContainerPrometheusMetricsPathEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PATH"
	QueueProxyAggregatePrometheusMetricsPortEnvVarKey = "AGGREGATE_PROMETHEUS_METRICS_PORT"
	QueueProxyMetricsPort                             = "9091"
	DefaultQueueProxyMetricsPath                      = "/metrics"
	prometheusTimeoutHeader                           = "X-Prometheus-Scrape-Timeout-Seconds"
)

type ScrapeConfigurations struct {
	logger         *zap.Logger
	QueueProxyPath string `json:"path"`
	QueueProxyPort string `json:"port"`
	AppPort        string
	AppPath        string
}

type Logger = zap.Logger

func initializeLogger() *Logger {
	zaplogger, _ := zap.NewProduction()
	return zaplogger
}

func getURL(port string, path string) string {
	return fmt.Sprintf("http://localhost:%s%s", port, path)
}

// getHeaderTimeout parse a string like (1.234) representing number of seconds
func getHeaderTimeout(timeout string) (time.Duration, error) {
	timeoutSeconds, err := strconv.ParseFloat(timeout, 64)
	if err != nil {
		return 0 * time.Second, err
	}

	return time.Duration(timeoutSeconds * 1e9), nil
}

func applyHeaders(into http.Header, from http.Header, keys ...string) {
	for _, key := range keys {
		val := from.Get(key)
		if val != "" {
			into.Set(key, val)
		}
	}
}

func getServerlessLabelVals() []string {
	var labelValues []string
	for _, envVar := range EnvVars {
		labelValues = append(labelValues, os.Getenv(envVar))
	}
	return labelValues
}

// addServerlessLabels adds the serverless labels to the prometheus metrics that are imported in from the application.
// this is done so that the prometheus metrics (both queue-proxy's and main-container's) can be easily queried together.
func addServerlessLabels(metric *ioprometheusclient.Metric, labelKeys []string, labelValues []string) *ioprometheusclient.Metric {
	// LabelKeys, EnvVars, and LabelVals are []string to enforce setting them in order (helps with testing)
	for idx, name := range labelKeys {
		labelName := name
		labelValue := labelValues[idx]
		newLabelPair := &ioprometheusclient.LabelPair{
			Name:  &labelName,
			Value: &labelValue,
		}
		metric.Label = append(metric.Label, newLabelPair)
	}
	return metric
}

// sanitizeMetrics attempts to convert UNTYPED metrics into either a gauge or counter.
// counter metric names with _created and gauge metric names with _total are converted due to irregularities
// observed in the conversion of these metrics from text to metric families.
func sanitizeMetrics(mf *ioprometheusclient.MetricFamily) *ioprometheusclient.MetricFamily {
	if strings.HasSuffix(*mf.Name, "_created") {
		counter := ioprometheusclient.MetricType_COUNTER
		var newMetric []*ioprometheusclient.Metric
		for _, metric := range mf.Metric {
			newMetric = append(newMetric, &ioprometheusclient.Metric{
				Label: metric.Label,
				Counter: &ioprometheusclient.Counter{
					Value: metric.Untyped.Value,
				},
				TimestampMs: metric.TimestampMs,
			})
		}
		return &ioprometheusclient.MetricFamily{
			Name:   mf.Name,
			Help:   mf.Help,
			Type:   &counter,
			Metric: newMetric,
		}
	}

	if strings.HasSuffix(*mf.Name, "_total") {
		gauge := ioprometheusclient.MetricType_GAUGE
		var newMetric []*ioprometheusclient.Metric
		for _, metric := range mf.Metric {
			newMetric = append(newMetric, &ioprometheusclient.Metric{
				Label: metric.Label,
				Gauge: &ioprometheusclient.Gauge{
					Value: metric.Untyped.Value,
				},
				TimestampMs: metric.TimestampMs,
			})
		}
		return &ioprometheusclient.MetricFamily{
			Name:   mf.Name,
			Help:   mf.Help,
			Type:   &gauge,
			Metric: newMetric,
		}
	}
	return nil
}

func scrapeAndWriteAppMetrics(mfs map[string]*ioprometheusclient.MetricFamily, w io.Writer, format expfmt.Format, logger *zap.Logger) error {
	var errs error
	labelValues := getServerlessLabelVals()

	for _, metricFamily := range mfs {
		var newMetric []*ioprometheusclient.Metric
		var mf *ioprometheusclient.MetricFamily

		// Some metrics from main-container are UNTYPED. This can cause errors in the promtheus scraper.
		// These metrics seem to be either gauges or counters. For now, avoid these errors by sanitizing the metrics
		// based on the metric name. If the metric can't be converted, we log an error. In the future, we should
		// figure out the root cause of this. (Possibly due to open metrics being read in as text and converted to MetricFamily)
		if *metricFamily.Type == ioprometheusclient.MetricType_UNTYPED {
			mf = sanitizeMetrics(metricFamily)
			if mf == nil {
				// if the metric fails to convert, discard it and keep exporting the rest of the metrics
				logger.Error("failed to parse untyped metric", zap.Any("metric name", metricFamily.Name))
				continue
			}
		} else {
			mf = metricFamily
		}

		// create a new list of Metric with the added serverless labels to each individual Metric
		for _, metric := range mf.Metric {
			m := addServerlessLabels(metric, LabelKeys, labelValues)
			newMetric = append(newMetric, m)
		}
		mf.Metric = newMetric

		_, err := expfmt.MetricFamilyToText(w, mf)
		if err != nil {
			logger.Error("multierr", zap.Error(err))
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// scrape sends a request to the provided url to scrape metrics from
// This will attempt to mimic some of Prometheus functionality by passing some headers through
// scrape returns the scraped metrics reader as well as the response's "Content-Type" header to determine the metrics format
func scrape(url string, header http.Header, logger *zap.Logger) (io.ReadCloser, context.CancelFunc, string, error) {
	var cancel context.CancelFunc
	ctx := context.Background()
	if timeoutString := header.Get(prometheusTimeoutHeader); timeoutString != "" {
		timeout, err := getHeaderTimeout(timeoutString)
		if err != nil {
			logger.Error("Failed to parse timeout header", zap.Error(err), zap.String("timeout", timeoutString))
		} else {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, cancel, "", err
	}

	applyHeaders(req.Header, header, "Accept",
		"User-Agent",
		prometheusTimeoutHeader,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, cancel, "", fmt.Errorf("error scraping %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			cancel()
			return nil, nil, "", err
		}
		return nil, cancel, "", fmt.Errorf("error scraping %s, status code: %v", url, resp.StatusCode)
	}
	format := resp.Header.Get("Content-Type")
	return resp.Body, cancel, format, nil
}

func NewScrapeConfigs(logger *zap.Logger, queueProxyPort string, appPort string, appPath string) *ScrapeConfigurations {
	return &ScrapeConfigurations{
		logger:         logger,
		QueueProxyPath: DefaultQueueProxyMetricsPath,
		QueueProxyPort: queueProxyPort,
		AppPort:        appPort,
		AppPath:        appPath,
	}
}

func (sc *ScrapeConfigurations) handleStats(w http.ResponseWriter, r *http.Request) {
	var err error
	var queueProxy, application io.ReadCloser
	var queueProxyCancel, appCancel context.CancelFunc

	defer func() {
		if queueProxy != nil {
			err = queueProxy.Close()
			if err != nil {
				sc.logger.Error("queue proxy connection is not closed", zap.Error(err))
			}
		}
		if application != nil {
			err = application.Close()
			if err != nil {
				sc.logger.Error("application connection is not closed", zap.Error(err))
			}
		}
		if queueProxyCancel != nil {
			queueProxyCancel()
		}
		if appCancel != nil {
			appCancel()
		}
	}()

	// Gather all the metrics we will merge
	if sc.QueueProxyPort != "" {
		queueProxyURL := getURL(sc.QueueProxyPort, sc.QueueProxyPath)
		if queueProxy, queueProxyCancel, _, err = scrape(queueProxyURL, r.Header, sc.logger); err != nil {
			sc.logger.Error("failed scraping queue proxy metrics", zap.Error(err))
		}
	}

	// Scrape app metrics if defined
	if sc.AppPort != "" {
		containerURL := getURL(sc.AppPort, sc.AppPath)
		var contentType string
		if application, appCancel, contentType, err = scrape(containerURL, r.Header, sc.logger); err != nil {
			sc.logger.Error("failed scraping application metrics", zap.Error(err), zap.String("content type", contentType))
		}
	}

	// Since we convert the scraped metrics to text, set the format as text even if
	// the content type is originally open metrics.
	format := expfmt.NewFormat(expfmt.TypeTextPlain)
	w.Header().Set("Content-Type", string(format))

	if queueProxy != nil {
		_, err = io.Copy(w, queueProxy)
		if err != nil {
			sc.logger.Error("failed to scraping and writing queue proxy metrics", zap.Error(err))
		}
	}

	if application != nil {
		var err error
		var parser expfmt.TextParser
		var mfs map[string]*ioprometheusclient.MetricFamily
		mfs, err = parser.TextToMetricFamilies(application)
		if err != nil {
			sc.logger.Error("error converting text to metric families", zap.Error(err), zap.Any("metric families return value", mfs))
		}
		if err = scrapeAndWriteAppMetrics(mfs, w, format, sc.logger); err != nil {
			sc.logger.Error("failed scraping and writing metrics", zap.Error(err))
		}
	}
}

func main() {
	zapLogger := initializeLogger()
	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	aggregateMetricsPort := os.Getenv(QueueProxyAggregatePrometheusMetricsPortEnvVarKey)
	sc := NewScrapeConfigs(
		zapLogger,
		QueueProxyMetricsPort,
		os.Getenv(ContainerPrometheusMetricsPortEnvVarKey),
		os.Getenv(ContainerPrometheusMetricsPathEnvVarKey),
	)
	mux.HandleFunc(`/metrics`, sc.handleStats)
	l, err := net.Listen("tcp", fmt.Sprintf(":%v", aggregateMetricsPort))
	if err != nil {
		zapLogger.Error("error listening on status port", zap.Error(err))
		return
	}

	errCh := make(chan error)
	go func() {
		zapLogger.Info(fmt.Sprintf("Starting stats server on port %v", aggregateMetricsPort))
		if err = http.Serve(l, mux); err != nil {
			errCh <- fmt.Errorf("stats server failed to serve: %w", err)
		}
	}()

	go func() {
		if err := sharedmain.Main(); err != nil {
			errCh <- err
		}
		// sharedMain exited without error which means graceful shutdown due to SIGTERM / SIGINT signal
		// Attempt a graceful shutdown of stats server
		cancel()
	}()

	// Blocks until sharedMain or server exits unexpectedly or SIGTERM / SIGINT signal is received.
	select {
	case err := <-errCh:
		zapLogger.Error("error serving aggregate metrics", zap.Error(err))
		os.Exit(1)

	case <-ctx.Done():
		zapLogger.Info("Attempting graceful shutdown of stats server")
		err := l.Close()
		if err != nil {
			zapLogger.Error("failed to shutdown stats server", zap.Error(err))
			os.Exit(1)
		}
	}
	zapLogger.Info("Stats server has successfully terminated")
}
