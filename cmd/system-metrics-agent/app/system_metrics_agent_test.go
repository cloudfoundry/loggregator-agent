package app_test

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/collector"

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
		inputFunc := func() (collector.SystemStat, error) {
			return defaultStat, nil
		}

		agent = app.NewSystemMetricsAgent(
			inputFunc,
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

var (
	defaultStat = collector.SystemStat{
		MemKB:      1025,
		MemPercent: 10.01,

		SwapKB:      2049,
		SwapPercent: 20.01,

		Load1M:  1.1,
		Load5M:  5.5,
		Load15M: 15.15,

		CPUStat: collector.CPUStat{
			User:   25.25,
			System: 52.52,
			Idle:   10.10,
			Wait:   22.22,
		},

		SystemDisk: collector.DiskStat{
			Present: true,

			Percent:      35.0,
			InodePercent: 45.0,

			ReadBytes:  10,
			WriteBytes: 20,
			ReadTime:   30,
			WriteTime:  40,
			IOTime:     50,
		},

		EphemeralDisk: collector.DiskStat{
			Present: true,

			Percent:      55.0,
			InodePercent: 65.0,

			ReadBytes:  100,
			WriteBytes: 200,
			ReadTime:   300,
			WriteTime:  400,
			IOTime:     500,
		},

		PersistentDisk: collector.DiskStat{
			Present: true,

			Percent:      75.0,
			InodePercent: 85.0,

			ReadBytes:  1000,
			WriteBytes: 2000,
			ReadTime:   3000,
			WriteTime:  4000,
			IOTime:     5000,
		},

		ProtoCounters: collector.ProtoCountersStat{
			Present:         true,
			IPForwarding:    1,
			UDPNoPorts:      2,
			UDPInErrors:     3,
			UDPLiteInErrors: 4,
			TCPActiveOpens:  5,
			TCPCurrEstab:    6,
			TCPRetransSegs:  7,
		},

		Health: collector.HealthStat{
			Present: true,
			Healthy: true,
		},
	}
)
