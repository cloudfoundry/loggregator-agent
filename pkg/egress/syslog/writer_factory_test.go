package syslog_test

import (
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

var _ = Describe("EgressFactory", func() {
	var (
		f       syslog.WriterFactory
		sm      *spyMetrics
		skipSSL = false
	)

	BeforeEach(func() {
		sm = newSpyMetrics()
		f = syslog.NewWriterFactory(sm)
	})

	It("returns an https writer when the url begins with https", func() {
		url, err := url.Parse("https://the-syslog-endpoint.com")
		Expect(err).ToNot(HaveOccurred())
		urlBinding := &syslog.URLBinding{
			URL: url,
		}

		writer, err := f.NewWriter(urlBinding, syslog.NetworkTimeoutConfig{}, skipSSL)
		Expect(err).ToNot(HaveOccurred())

		_, ok := writer.(*syslog.HTTPSWriter)
		Expect(ok).To(BeTrue())

		Expect(sm.names).To(ConsistOf("Egress"))
	})

	It("returns a tcp writer when the url begins with syslog://", func() {
		url, err := url.Parse("syslog://the-syslog-endpoint.com")
		Expect(err).ToNot(HaveOccurred())
		urlBinding := &syslog.URLBinding{
			URL: url,
		}

		writer, err := f.NewWriter(urlBinding, syslog.NetworkTimeoutConfig{}, skipSSL)
		Expect(err).ToNot(HaveOccurred())

		_, ok := writer.(*syslog.TCPWriter)
		Expect(ok).To(BeTrue())

		Expect(sm.names).To(ConsistOf("Egress"))
	})

	It("returns a syslog-tls writer when the url begins with syslog-tls://", func() {
		url, err := url.Parse("syslog-tls://the-syslog-endpoint.com")
		Expect(err).ToNot(HaveOccurred())
		urlBinding := &syslog.URLBinding{
			URL: url,
		}

		writer, err := f.NewWriter(urlBinding, syslog.NetworkTimeoutConfig{}, skipSSL)
		Expect(err).ToNot(HaveOccurred())

		_, ok := writer.(*syslog.TLSWriter)
		Expect(ok).To(BeTrue())
		Expect(sm.names).To(ConsistOf("Egress"))
	})

	It("returns an error when given a binding with an invalid scheme", func() {
		url, err := url.Parse("invalid://the-syslog-endpoint.com")
		Expect(err).ToNot(HaveOccurred())
		urlBinding := &syslog.URLBinding{
			URL: url,
		}

		_, err = f.NewWriter(urlBinding, syslog.NetworkTimeoutConfig{}, skipSSL)
		Expect(err).To(MatchError("unsupported protocol"))
	})
})

type spyMetrics struct {
	names        []string
	metricValues chan float64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		metricValues: make(chan float64, 100),
	}
}

func (sm *spyMetrics) NewCounter(name string) func(uint64) {
	sm.names = append(sm.names, name)
	return func(val uint64) {
		sm.metricValues <- float64(val)
	}
}
