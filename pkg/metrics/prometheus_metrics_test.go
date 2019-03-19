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

		g, err := r.NewGauge(
			"test_gauge",
			metrics.WithHelpText("a gauge help text for test_gauge"),
			metrics.WithMetricTags(map[string]string{"bar": "baz"}),
		)

		c.Add(10)
		g.Set(10)
		g.Add(1)

		addr := fmt.Sprintf("http://127.0.0.1:%s/metrics", port)
		resp, err := http.Get(addr)
		Expect(err).ToNot(HaveOccurred())

		respBytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		response := string(respBytes)
		Expect(response).To(ContainSubstring(`test_counter{foo="bar",origin="test-source",source_id="test-source"} 10`))
		Expect(response).To(ContainSubstring("a counter help text for test_counter"))
		Expect(response).To(ContainSubstring(`test_gauge{bar="baz",origin="test-source",source_id="test-source"} 11`))
		Expect(response).To(ContainSubstring("a gauge help text for test_gauge"))
	})

	It("returns an error if the metric is invalid", func() {
		r := metrics.NewPromRegistry("test-source", 0, l)

		_, err := r.NewCounter("test-counter")
		Expect(err).To(HaveOccurred())

		_, err = r.NewGauge("test-gauge")
		Expect(err).To(HaveOccurred())
	})
})
