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

		c, err := r.NewCounter(
			"test_counter",
			metrics.WithMetricTags(map[string]string{"foo": "bar"}),
			metrics.WithHelpText("a counter help text for test_counter"),
		)
		Expect(err).ToNot(HaveOccurred())

		g, err := r.NewGauge(
			"test_gauge",
			metrics.WithHelpText("a gauge help text for test_gauge"),
			metrics.WithMetricTags(map[string]string{"bar": "baz"}),
		)
		Expect(err).ToNot(HaveOccurred())

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

		_, err := r.NewCounter(
			"test_counter",
			metrics.WithHelpText("a counter help text for test_counter"),
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = r.NewGauge(
			"test_gauge",
			metrics.WithHelpText("a gauge help text for test_gauge"),
		)
		Expect(err).ToNot(HaveOccurred())

		resp := getMetrics(r.Port())
		Expect(resp).To(ContainSubstring(`test_counter{origin="test-source",source_id="test-source",tag="custom"} 0`))
		Expect(resp).To(ContainSubstring(`test_gauge{origin="test-source",source_id="test-source",tag="custom"} 0`))
	})

	It("returns an error if the metric is invalid", func() {
		r := metrics.NewPromRegistry("test-source", 0, l)

		_, err := r.NewCounter("test-counter")
		Expect(err).To(HaveOccurred())

		_, err = r.NewGauge("test-gauge")
		Expect(err).To(HaveOccurred())
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
