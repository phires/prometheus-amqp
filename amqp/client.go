package amqp

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"math"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/model"
	"pack.ag/amqp"
)

// Client allows sending batches of Prometheus samples to AMQP.
type Client struct {
	logger log.Logger

	address     string
	username    string
	password    string
	timeout     time.Duration
	queue       string
	connOptions []amqp.ConnOption
	connection  *amqp.Client
	session     *amqp.Session
}

type Metric struct {
	name      string
	timestamp float64
	value     float64
}

// NewClient creates a new Client.
func NewClient(logger log.Logger, address string, queue string, user string, password string, timeout time.Duration) (*Client, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	c := &Client{
		logger:   logger,
		address:  address,
		username: user,
		password: password,
		queue:    queue,
		timeout:  timeout,
	}

	if len(c.username) > 0 && len(c.password) > 0 {
		c.connOptions = append(c.connOptions, amqp.ConnSASLPlain(c.username, c.password))
	}

	return c, nil
}

// LogWrite only logs the samples without sending them to anywhere
func (c *Client) WriteLog(samples model.Samples) error {
	for _, s := range samples {
		k := pathFromMetric(s.Metric)
		//t := float64(s.Timestamp.UnixNano()) / 1e9
		v := float64(s.Value)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		level.Debug(c.logger).Log("msg", "sample-metric", "path", k, "value", v)
	}

	return nil
}

// Write sends a batch of samples to AMQP.
func (c *Client) Write(samples model.Samples) error {

	//var buf bytes.Buffer
	ctx := context.Background()

	client, err := amqp.Dial(c.address, c.connOptions...)
	if err != nil {
		return fmt.Errorf("Error dailing AMQP server: %v", err)
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Error creating AMQP session: %v", err)
	}
	// level.Debug(c.logger).Log("msg", "amqp connection built", "address", c.address, "queue", c.queue)

	sender, err := session.NewSender(amqp.LinkTargetAddress(c.queue))
	if err != nil {
		level.Error(c.logger).Log("msg", "error linking to amqp", "queue", c.queue, "error", err)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)

	for _, s := range samples {
		//k := pathFromMetric(s.Metric)
		//t := float64(s.Timestamp.UnixNano()) / 1e9
		v := float64(s.Value)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}

		data, err := s.MarshalJSON()

		if err != nil {
			level.Error(c.logger).Log("msg", "error json", "value", v, "sample", s)
			continue
		}

		err = sender.Send(ctx, amqp.NewMessage(data))
		if err != nil {
			level.Error(c.logger).Log("msg", "error sending message", "error", err)
		}

	}
	session.Close(ctx)
	client.Close()

	cancel()
	return nil
}

func pathFromMetric(m model.Metric) string {
	var buffer bytes.Buffer

	buffer.WriteString(string(m[model.MetricNameLabel]))

	// We want to sort the labels.
	labels := make(model.LabelNames, 0, len(m))
	for l := range m {
		labels = append(labels, l)
	}
	sort.Sort(labels)

	// For each label, in order, add ".<label>.<value>".
	for _, l := range labels {
		v := m[l]

		if l == model.MetricNameLabel || len(l) == 0 {
			continue
		}
		// Since we use '.' instead of '=' to separate label and values
		// it means that we can't have an '.' in the metric name. Fortunately
		// this is prohibited in prometheus metrics.
		buffer.WriteString(fmt.Sprintf(
			".%s.%s", string(l), v))
	}
	return buffer.String()
}

// Name identifies the client as a amqp client.
func (c Client) Name() string {
	return "amqp"
}
