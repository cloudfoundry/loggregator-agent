package app_test

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/system-metrics-agent/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemMetricsAgent", func() {
	var (
		agent *app.SystemMetricsAgent
	)

	BeforeEach(func() {
		agent = app.NewSystemMetricsAgent(
			app.Config{
				SampleInterval: time.Millisecond,
				Deployment:     "some-deployment",
				Job:            "some-job",
				Index:          "some-index",
				IP:             "some-ip",
			},
			log.New(GinkgoWriter, "", log.LstdFlags),
		)
	})

	It("has an http listener for PProf", func() {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.DebugAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		resp, err := http.Get("http://" + addr + "/debug/pprof/")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("has a prom exposition endpoint", func() {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.MetricsAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		resp, err := http.Get("http://" + addr + "/metrics")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	DescribeTable("default prom labels", func(label string) {
		go agent.Run()
		defer agent.Shutdown(context.Background())

		var addr string
		Eventually(func() int {
			addr = agent.MetricsAddr()
			return len(addr)
		}).ShouldNot(Equal(0))

		Eventually(hasLabel(addr, label)).Should(BeTrue())
	},
		Entry("origin", `origin="system_metrics_agent"`),
		Entry("source_id", `source_id="system_metrics_agent"`),
		Entry("deployment", `deployment="some-deployment"`),
		Entry("job", `job="some-job"`),
		Entry("index", `index="some-index"`),
		Entry("ip", `ip="some-ip"`),
	)
})

func hasLabel(addr, label string) func() bool {
	return func() bool {
		resp, err := http.Get("http://" + addr + "/metrics")
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		return strings.Contains(string(body), label)
	}
}
