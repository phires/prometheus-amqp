package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/phires/prometheus-amqp/amqp"
	"github.com/phires/prometheus-amqp/filter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promlog"

	"github.com/prometheus/prometheus/prompb"
)

type config struct {
	amqpAddress       string
	amqpAccessKey     string
	amqpAccessKeyName string
	amqpQueueName     string

	remoteTimeout time.Duration
	listenAddr    string
	telemetryPath string

	logOnly bool

	pathFilterConfigFile string
}

var (
	receivedSamples = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "received_samples_total",
			Help: "Total number of received samples.",
		},
	)
	sentSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sent_samples_total",
			Help: "Total number of processed samples sent to remote storage.",
		},
		[]string{"remote"},
	)
	failedSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_samples_total",
			Help: "Total number of processed samples which failed on send to remote storage.",
		},
		[]string{"remote"},
	)
	sentBatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sent_batch_duration_seconds",
			Help:    "Duration of sample batch send calls to the remote storage.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"remote"},
	)
)

func init() {
	prometheus.MustRegister(receivedSamples)
	prometheus.MustRegister(sentSamples)
	prometheus.MustRegister(failedSamples)
	prometheus.MustRegister(sentBatchDuration)
}

type writer interface {
	Write(samples model.Samples) error
	WriteLog(samples model.Samples) error
	Name() string
}

type reader interface {
	Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error)
	Name() string
}

func parseFlags() *config {
	cfg := &config{}

	flag.StringVar(&cfg.amqpAccessKey, "amqp-accesskey", "",
		"The plain SASL access key. None, if empty.",
	)
	flag.StringVar(&cfg.amqpAccessKeyName, "amqp-accesskeyname", "",
		"The plain SASL access key name. None, if empty.",
	)
	flag.StringVar(&cfg.amqpAddress, "amqp-address", "",
		"URL of the AMQP. None, if empty.",
	)
	flag.StringVar(&cfg.amqpQueueName, "amqp-queue", "",
		"AMQP queue name. None, if empty.",
	)
	flag.BoolVar(&cfg.logOnly, "log-only", false,
		"If flag is set the metric samples will only be written to the log.",
	)
	flag.StringVar(&cfg.pathFilterConfigFile, "filter-file", "",
		"Filter file to use. If empty, filter is not used")

	flag.DurationVar(&cfg.remoteTimeout, "send-timeout", 30*time.Second,
		"The timeout to use when sending samples to the remote storage.",
	)
	flag.StringVar(&cfg.listenAddr, "web.listen-address", ":24282", "Address to listen on for web endpoints.")
	flag.StringVar(&cfg.telemetryPath, "web.telemetry-path", "/metrics", "Address to listen on for web endpoints.")

	flag.Parse()

	return cfg
}

func buildClients(logger log.Logger, cfg *config) ([]writer, []reader) {
	var writers []writer
	var readers []reader
	if cfg.amqpAddress != "" {
		level.Debug(logger).Log("msg", "AMQP writer", "amqpAddress", cfg.amqpAddress, "amqpQueueName", cfg.amqpQueueName)

		c, err := amqp.NewClient(
			log.With(logger, "storage", "amqp"),
			cfg.amqpAddress, cfg.amqpQueueName,
			cfg.amqpAccessKey, cfg.amqpAccessKeyName,
			cfg.remoteTimeout)
		if err != nil {
			fmt.Printf("couldn't build writer")
		}
		writers = append(writers, c)
	}

	level.Info(logger).Log("Starting up...")
	return writers, readers
}

func serve(logger log.Logger, addr string, writers []writer, readers []reader, cfg *config) error {
	http.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			level.Error(logger).Log("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			level.Error(logger).Log("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			level.Error(logger).Log("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		samples := protoToSamples(logger, &req)
		receivedSamples.Add(float64(len(samples)))

		var wg sync.WaitGroup
		for _, w := range writers {
			wg.Add(1)
			go func(rw writer) {
				sendSamples(logger, rw, samples, cfg)
				wg.Done()
			}(w)
		}
		wg.Wait()
	})

	return http.ListenAndServe(addr, nil)
}

func protoToSamples(logger log.Logger, req *prompb.WriteRequest) model.Samples {
	var samples model.Samples
	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			labelName := model.LabelName(l.Name)
			metric[labelName] = model.LabelValue(l.Value)
		}

		// Filter here
		keep, err := filter.MatchesFilter(metric, logger)

		if err != nil {
			level.Warn(logger).Log("msg", "Filter failure", "err", err)
		}
		if !keep {
			continue
		}

		for _, s := range ts.Samples {
			samples = append(samples, &model.Sample{
				Metric:    metric,
				Value:     model.SampleValue(s.Value),
				Timestamp: model.Time(s.Timestamp),
			})
		}
	}
	return samples
}

func sendSamples(logger log.Logger, w writer, samples model.Samples, cfg *config) {
	begin := time.Now()

	var err error
	if cfg.logOnly {
		err = w.WriteLog(samples)
	} else {
		err = w.Write(samples)
	}

	duration := time.Since(begin).Seconds()
	if err != nil {
		level.Warn(logger).Log("msg", "Error sending samples to remote storage", "err", err, "storage", w.Name(), "num_samples", len(samples))
		failedSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	}
	sentSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	sentBatchDuration.WithLabelValues(w.Name()).Observe(duration)
}

func main() {
	cfg := parseFlags()
	http.Handle(cfg.telemetryPath, prometheus.Handler())

	logLevel := promlog.AllowedLevel{}
	logLevel.Set("debug")

	logger := promlog.New(logLevel)

	err := filter.Init(cfg.pathFilterConfigFile)
	if err != nil {
		level.Error(logger).Log("msg", "Failed reading metric filter", "file", cfg.pathFilterConfigFile, "err", err)
		os.Exit(-1)
	}
	level.Debug(logger).Log("msg", "Metric filter", "file", cfg.pathFilterConfigFile, "count", filter.Count())
	writers, readers := buildClients(logger, cfg)
	level.Debug(logger).Log("msg", "Start listening", "listenAddr", cfg.listenAddr)
	if cfg.logOnly {
		level.Debug(logger).Log("msg", "Logging only, not sending anything to queue!")
	}

	serve(logger, cfg.listenAddr, writers, readers, cfg)
}
