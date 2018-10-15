package app_test

import (
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent/app"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ForwarderAgent", func() {
	var (
		bf *stubBindingFetcher
		sm *spyMetrics
	)

	BeforeEach(func() {
		bf = newStubBindingFetcher()
		sm = newSpyMetrics()
	})

	It("reports the number of binding that come from the Getter", func() {
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-2", "host-2", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}

		fa := app.NewForwarderAgent(0, sm, bf, 100*time.Millisecond)
		fa.Run(false)

		var mv float64
		Eventually(sm.metricValues).Should(Receive(&mv))
		Expect(mv).To(BeNumerically("==", 3))
	})

	It("polls for updates from the binding fetcher and updates the metric accordingly", func() {
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}
		bf.bindings <- []cups.Binding{
			{"app-1", "host-1", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
			{"app-3", "host-3", "v3-syslog://drain.url.com"},
		}

		fa := app.NewForwarderAgent(0, sm, bf, 100*time.Millisecond)
		fa.Run(false)

		Eventually(sm.metricValues).Should(HaveLen(2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 3))
	})

})

type stubBindingFetcher struct {
	bindings chan []cups.Binding
}

func newStubBindingFetcher() *stubBindingFetcher {
	return &stubBindingFetcher{
		bindings: make(chan []cups.Binding, 100),
	}
}

func (s *stubBindingFetcher) FetchBindings() ([]cups.Binding, error) {
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
