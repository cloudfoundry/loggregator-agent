package metrics_test

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"log"
	"net/http"
)

var _ = Describe("PrometheusMetrics", func() {

	var (
		l *log.Logger
	)

	BeforeEach(func() {
		l = log.New(GinkgoWriter, "", log.LstdFlags)
	})

	It("serves metrics on a prometheus endpoint", func() {
		r := metrics.NewPromRegistry("test-source", 0, l)
		port := r.Port()

		c := r.NewCounter(
			"test_counter",
			metrics.WithMetricTags(map[string]string{"foo": "bar"}),
			metrics.WithHelpText("a counter help text for test_counter"),
		)

		g := r.NewGauge(
			"test_gauge",
			metrics.WithHelpText("a gauge help text for test_gauge"),
			metrics.WithMetricTags(map[string]string{"bar": "baz"}),
		)

		c.Add(10)
		g.Set(10)
		g.Add(1)

		resp := getMetrics(port)
		Expect(resp).To(ContainSubstring(`test_counter{foo="bar",origin="test-source",source_id="test-source"} 10`))
		Expect(resp).To(ContainSubstring("a counter help text for test_counter"))
		Expect(resp).To(ContainSubstring(`test_gauge{bar="baz",origin="test-source",source_id="test-source"} 11`))
		Expect(resp).To(ContainSubstring("a gauge help text for test_gauge"))
	})

	It("accepts custom default tags", func() {
		ct := map[string]string{
			"tag": "custom",
		}

		r := metrics.NewPromRegistry("test-source", 0, l, metrics.WithDefaultTags(ct))

		r.NewCounter(
			"test_counter",
			metrics.WithHelpText("a counter help text for test_counter"),
		)

		r.NewGauge(
			"test_gauge",
			metrics.WithHelpText("a gauge help text for test_gauge"),
		)

		resp := getMetrics(r.Port())
		Expect(resp).To(ContainSubstring(`test_counter{origin="test-source",source_id="test-source",tag="custom"} 0`))
		Expect(resp).To(ContainSubstring(`test_gauge{origin="test-source",source_id="test-source",tag="custom"} 0`))
	})

	It("panics if the metric is invalid", func() {
		r := metrics.NewPromRegistry("test-source", 0, l)

		Expect(func() {
			r.NewCounter("test-counter")
		}).To(Panic())

		Expect(func() {
			r.NewGauge("test-counter")
		}).To(Panic())
	})
})

func getMetrics(port string) string {
	addr := fmt.Sprintf("http://127.0.0.1:%s/metrics", port)
	resp, err := http.Get(addr)
	Expect(err).ToNot(HaveOccurred())
	respBytes, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	return string(respBytes)
}
