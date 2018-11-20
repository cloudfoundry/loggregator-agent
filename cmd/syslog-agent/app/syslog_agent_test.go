package app_test

import (
	"log"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/syslog-agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SyslogAgent", func() {
	var (
		bf   *stubBindingFetcher
		sm   *spyMetrics
		port = uint16(11000)
	)

	BeforeEach(func() {
		bf = newStubBindingFetcher()
		sm = newSpyMetrics()
	})

	AfterEach(func() {
		port++
	})

	It("reports the number of binding that come from the Getter", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-2", "host-2", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}

		fa := app.NewSyslogAgent(
			0,
			sm,
			bf,
			100*time.Millisecond,
			app.GRPC{
				Port:     port,
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
			false,
			log.New(GinkgoWriter, "", 0),
		)
		fa.Run(false)

		var mv float64
		Eventually(sm.metricValues).Should(Receive(&mv))
		Expect(mv).To(BeNumerically("==", 3))
	})

	It("polls for updates from the binding fetcher and updates the metric accordingly", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}

		fa := app.NewSyslogAgent(
			0,
			sm,
			bf,
			100*time.Millisecond,
			app.GRPC{
				Port:     port,
				CAFile:   testhelper.Cert("loggregator-ca.crt"),
				CertFile: testhelper.Cert("metron.crt"),
				KeyFile:  testhelper.Cert("metron.key"),
			},
			false,
			log.New(GinkgoWriter, "", 0),
		)
		fa.Run(false)

		Eventually(sm.metricValues).Should(Receive(Equal(2.0)))
		Eventually(sm.metricValues).Should(Receive(Equal(3.0)))
	})

})

type stubBindingFetcher struct {
	bindings chan []syslog.Binding
}

func newStubBindingFetcher() *stubBindingFetcher {
	return &stubBindingFetcher{
		bindings: make(chan []syslog.Binding, 100),
	}
}

func (s *stubBindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	select {
	case b := <-s.bindings:
		return b, nil
	default:
		return nil, nil
	}
}

type spyMetrics struct {
	name         string
	metricValues chan float64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		metricValues: make(chan float64, 100),
	}
}

func (sm *spyMetrics) NewGauge(name string) func(float64) {
	sm.name = name
	return func(val float64) {
		sm.metricValues <- val
	}
}

func (sm *spyMetrics) NewCounter(name string) func(uint64) {
	sm.name = name
	return func(val uint64) {
		sm.metricValues <- float64(val)
	}
}
