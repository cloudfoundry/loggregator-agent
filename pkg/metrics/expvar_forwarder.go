package metrics

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"

	"google.golang.org/grpc"
)

// ExpvarForwarder reads from an expvar and write them to LogCache.
type ExpvarForwarder struct {
	log             *log.Logger
	interval        time.Duration
	defaultSourceId string

	// LogCache
	logCacheAddr string
	opts         []grpc.DialOption
	globalTags   map[string]string

	metrics map[string][]metricInfo
}

// NewExpvarForwarder returns a new ExpvarForwarder.
func NewExpvarForwarder(logCacheAddr string, opts ...ExpvarForwarderOption) *ExpvarForwarder {
	f := &ExpvarForwarder{
		log:      log.New(ioutil.Discard, "", 0),
		interval: time.Minute,

		logCacheAddr: logCacheAddr,
		opts:         []grpc.DialOption{grpc.WithInsecure()},
		globalTags:   make(map[string]string),

		metrics: make(map[string][]metricInfo),
	}

	for _, o := range opts {
		o(f)
	}

	return f
}

// ExpvarForwarderOption configures an ExpvarForwarder.
type ExpvarForwarderOption func(*ExpvarForwarder)

// WithExpvarLogger returns an ExpvarForwarderOption that configures the logger
// used for the ExpvarForwarder. Defaults to silent logger.
func WithExpvarLogger(l *log.Logger) ExpvarForwarderOption {
	return func(f *ExpvarForwarder) {
		f.log = l
	}
}

// WithExpvarDialOpts returns an ExpvarForwarderOption that configures the dial
// options for dialing LogCache. Defaults to grpc.WithInsecure().
func WithExpvarDialOpts(opts ...grpc.DialOption) ExpvarForwarderOption {
	return func(f *ExpvarForwarder) {
		f.opts = opts
	}
}

// WithExpvarInterval returns an ExpvarForwarderOption that configures how often
// the ExpvarForwarder reads from the Expvar endpoints. Defaults to 1 minute.
func WithExpvarInterval(i time.Duration) ExpvarForwarderOption {
	return func(f *ExpvarForwarder) {
		f.interval = i
	}
}

func WithExpvarGlobalTag(key, value string) ExpvarForwarderOption {
	return func(f *ExpvarForwarder) {
		f.globalTags[key] = value
	}
}

func WithExpvarDefaultSourceId(sourceId string) ExpvarForwarderOption {
	return func(f *ExpvarForwarder) {
		f.defaultSourceId = sourceId
	}
}

// AddExpvarCounterTemplate returns an ExpvarForwarderOption that configures the
// ExpvarForwarder to look for counter metrics. Each template is a text/template.
// This can be called several times to add more counter metrics. There has to
// be atleast one counter or gauge template.
func AddExpvarCounterTemplate(addr, metricName, sourceId, txtTemplate string, tags map[string]string) ExpvarForwarderOption {
	t, err := template.New("Counter").Parse(txtTemplate)
	if err != nil {
		panic(err)
	}

	return func(f *ExpvarForwarder) {
		if sourceId == "" {
			sourceId = f.defaultSourceId
		}

		f.metrics[addr] = append(f.metrics[addr], metricInfo{
			name:       metricName,
			sourceId:   sourceId,
			template:   t,
			metricType: "counter",
			tags:       tags,
		})
	}
}

// WithExpvarGaugeTemplates returns an ExpvarForwarderOption that configures the
// ExpvarForwarder to look for gauge metrics. Each template is a text/template.
// This can be called several times to add more counter metrics. There has to
// be atleast one counter or gauge template.
func AddExpvarGaugeTemplate(addr, metricName, metricUnit, sourceId, txtTemplate string, tags map[string]string) ExpvarForwarderOption {
	t, err := template.New("Gauge").Parse(txtTemplate)
	if err != nil {
		panic(err)
	}

	return func(f *ExpvarForwarder) {
		if sourceId == "" {
			sourceId = f.defaultSourceId
		}

		f.metrics[addr] = append(f.metrics[addr], metricInfo{
			name:       metricName,
			unit:       metricUnit,
			sourceId:   sourceId,
			template:   t,
			metricType: "gauge",
			tags:       tags,
		})
	}
}

// Start starts the ExpvarForwarder. It starts reading from the given endpoints
// and looking for the corresponding metrics via the templates. Start blocks.
func (f *ExpvarForwarder) Start() {
	client, err := grpc.Dial(f.logCacheAddr, f.opts...)
	if err != nil {
		f.log.Panicf("failed to dial LogCache (%s): %s", f.logCacheAddr, err)
	}
	ingressClient := loggregator_v2.NewIngressClient(client)

	for range time.Tick(f.interval) {
		var e []*loggregator_v2.Envelope

		for addr, metrics := range f.metrics {
			resp, err := http.Get(addr)
			if err != nil {
				f.log.Printf("failed to read from %s: %s", addr, err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				f.log.Printf("Expected 200 but got %d from %s", resp.StatusCode, addr)
				continue
			}

			d := json.NewDecoder(resp.Body)
			d.UseNumber()

			var m map[string]interface{}
			if err := d.Decode(&m); err != nil {
				f.log.Printf("failed to unmarshal data from %s: %s", addr, err)
				continue
			}

			for _, metric := range metrics {
				b := &bytes.Buffer{}
				if err := metric.template.Execute(b, m); err != nil {
					f.log.Printf("failed to execute template: %s", err)
					continue
				}

				if metric.tags == nil {
					metric.tags = make(map[string]string)
				}

				for k, v := range f.globalTags {
					metric.tags[k] = v
				}

				if metric.metricType == "counter" {
					value, err := strconv.ParseUint(b.String(), 10, 64)
					if err != nil {
						f.log.Printf("counter result was not a uint64: %s", err)
						continue
					}

					now := time.Now().UnixNano()
					e = append(e, &loggregator_v2.Envelope{
						SourceId:  metric.sourceId,
						Timestamp: now,
						Tags:      metric.tags,
						Message: &loggregator_v2.Envelope_Counter{
							Counter: &loggregator_v2.Counter{
								Name:  metric.name,
								Total: value,
							},
						},
					})

					continue
				}

				if metric.metricType == "gauge" {
					value, err := strconv.ParseFloat(b.String(), 64)
					if err != nil {
						f.log.Printf("gauge result was not a float64: %s", err)
						continue
					}

					now := time.Now().UnixNano()
					e = append(e, &loggregator_v2.Envelope{
						SourceId:  metric.sourceId,
						Timestamp: now,
						Tags:      metric.tags,
						Message: &loggregator_v2.Envelope_Gauge{
							Gauge: &loggregator_v2.Gauge{
								Metrics: map[string]*loggregator_v2.GaugeValue{
									metric.name: {
										Value: value,
										Unit:  metric.unit,
									},
								},
							},
						},
					})

					continue
				}
			}
		}

		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := ingressClient.Send(ctx, &loggregator_v2.EnvelopeBatch{
			Batch: e,
		})
		if err != nil {
			f.log.Printf("failed to send metrics: %s", err)
			continue
		}
	}
}

type metricInfo struct {
	name       string
	unit       string
	sourceId   string
	template   *template.Template
	metricType string
	tags       map[string]string
}
