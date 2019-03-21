package app_test

import (
	"net"

	"code.cloudfoundry.org/loggregator-agent/cmd/agent/app"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("v2 App", func() {
	It("uses DopplerAddrWithAZ for AZ affinity", func() {
		spyLookup := newSpyLookup()

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
			clientCreds,
			serverCreds,
			testhelper.NewMetricClient(),
			app.WithV2Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(spyLookup.calledWith(expectedHost)).Should(BeTrue())
	})

	DescribeTable("has metric and expected tags", func(name string, tags map[string]string) {
		spyLookup := newSpyLookup()

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

		mc := testhelper.NewMetricClient()
		app := app.NewV2App(
			&config,
			clientCreds,
			serverCreds,
			mc,
			app.WithV2Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(func() bool {
			return mc.HasMetric(name, tags)
		}).Should(BeTrue())

		m := mc.GetMetric(name, tags)
		for k, v := range tags {
			Expect(m.Opts.ConstLabels).To(HaveKeyWithValue(k, v))
		}
	},
		Entry(
			"DroppedEgressV2",
			"dropped",
			map[string]string{"direction": "egress"}),
		Entry(
			"DroppedIngressV2",
			"dropped",
			map[string]string{"direction": "ingress"}),
		Entry(
			"EgressV2",
			"egress",
			map[string]string{}),
		Entry(
			"IngressV2",
			"ingress",
			map[string]string{}),
		Entry(
			"AverageEnvelopeV2",
			"average_envelope",
			map[string]string{}),
		Entry(
			"OriginMappingsV2",
			"origin_mappings",
			map[string]string{}),
	)

})
