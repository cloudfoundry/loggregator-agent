package syslog_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

var _ = Describe("RFC5424", func() {
	It("converts a log envelope to a slice of slice of byte in RFC5424 format", func() {
		env := buildLogEnvelope("MY TASK", "2", "just a test", loggregator_v2.Log_OUT)

		Expect(syslog.ToRFC5424(env, "test-hostname", "test-app-id")).To(Equal([][]byte{
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [MY-TASK/2] - - just a test\n"),
		}))
	})

	It("uses the correct priority for STDERR", func() {
		env := buildLogEnvelope("MY TASK", "2", "just a test", loggregator_v2.Log_ERR)

		Expect(syslog.ToRFC5424(env, "test-hostname", "test-app-id")).To(Equal([][]byte{
			[]byte("<11>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [MY-TASK/2] - - just a test\n"),
		}))
	})

	It("uses the correct priority for unknown log type", func() {
		env := buildLogEnvelope("MY TASK", "2", "just a test", 20)

		Expect(syslog.ToRFC5424(env, "test-hostname", "test-app-id")).To(Equal([][]byte{
			[]byte("<-1>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [MY-TASK/2] - - just a test\n"),
		}))
	})

	It("converts a gauge envelope to a slice of slice of byte in RFC5424 format", func() {
		env := buildGaugeEnvelope("1")

		result, err := syslog.ToRFC5424(env, "test-hostname", "test-app-id")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(ConsistOf(
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n"),
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"disk\" value=\"1234\" unit=\"bytes\"] \n"),
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"disk_quota\" value=\"1024\" unit=\"bytes\"] \n"),
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"memory\" value=\"5423\" unit=\"bytes\"] \n"),
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"memory_quota\" value=\"8000\" unit=\"bytes\"] \n"),
		))
	})

	It("converts a counter envelope to a slice of slice of byte in RFC5424 format", func() {
		env := buildCounterEnvelope("1")

		Expect(syslog.ToRFC5424(env, "test-hostname", "test-app-id")).To(Equal([][]byte{
			[]byte("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [counter@47450 name=\"some-counter\" total=\"99\" delta=\"1\"] \n"),
		}))
	})

	Describe("validation", func() {
		It("returns an error if hostname is longer than 255", func() {
			env := buildLogEnvelope("MY TASK", "2", "just a test", 20)
			_, err := syslog.ToRFC5424(env, invalidHostname, "test-app-id")
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if app name is longer than 48", func() {
			env := buildLogEnvelope("MY TASK", "2", "just a test", 20)
			_, err := syslog.ToRFC5424(env, "test-hostname", invalidAppName)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if process ID is longer than 128", func() {
			env := buildLogEnvelope("MY TASK", invalidProcessID, "just a test", 20)
			_, err := syslog.ToRFC5424(env, "test-hostname", "test-app-id")
			Expect(err).To(HaveOccurred())
		})
	})
})

var (
	invalidHostname  = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Cras tortor elit, ultricies in suscipit et, euismod quis velit. Duis et auctor mauris. Suspendisse et aliquet justo. Nunc fermentum lorem dolor, eu fermentum quam vulputate id. Morbi gravida ut elit sed."
	invalidAppName   = "Lorem ipsum dolor sit amet, consectetur posuere. HA!"
	invalidProcessID = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Cras tortor elit, ultricies in suscipit et, euismod quis velit. Duis et auctor mauris. Suspendisse et aliquet justo. Nunc fermentum lorem dolor,"
)
