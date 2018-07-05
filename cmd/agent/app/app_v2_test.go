package app_test

import (
	"net"

	"code.cloudfoundry.org/loggregator-agent/cmd/agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/healthendpoint"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

var _ = Describe("v2 App", func() {
	It("uses DopplerAddrWithAZ for AZ affinity", func() {
		spyLookup := newSpyLookup()
		gaugeMap := stubGaugeMap()

		promRegistry := prometheus.NewRegistry()
		he := healthendpoint.New(promRegistry, gaugeMap)
		clientCreds, err := plumbing.NewClientCredentials(
			testhelper.Cert("metron.crt"),
			testhelper.Cert("metron.key"),
			testhelper.Cert("loggregator-ca.crt"),
			"doppler",
		)
		Expect(err).ToNot(HaveOccurred())

		serverCreds, err := plumbing.NewServerCredentials(
			testhelper.Cert("router.crt"),
			testhelper.Cert("router.key"),
			testhelper.Cert("loggregator-ca.crt"),
		)
		Expect(err).ToNot(HaveOccurred())

		config := buildAgentConfig("127.0.0.1", 1234)
		config.Zone = "something-bad"
		expectedHost, _, err := net.SplitHostPort(config.RouterAddrWithAZ)
		Expect(err).ToNot(HaveOccurred())

		app := app.NewV2App(
			&config,
			he,
			clientCreds,
			serverCreds,
			testhelper.NewMetricClient(),
			app.WithV2Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(spyLookup.calledWith(expectedHost)).Should(BeTrue())
	})
})
