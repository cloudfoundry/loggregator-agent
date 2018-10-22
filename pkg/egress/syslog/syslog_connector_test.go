package syslog_test

import (
	"io"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SyslogConnector", func() {
	var (
		ctx          context.Context
		spyWaitGroup *SpyWaitGroup
		netConf      syslog.NetworkTimeoutConfig
	)

	BeforeEach(func() {
		ctx, _ = context.WithCancel(context.Background())
		spyWaitGroup = &SpyWaitGroup{}
	})

	It("connects to the passed syslog protocol", func() {
		var called bool
		constructor := func(
			*syslog.URLBinding,
			syslog.NetworkTimeoutConfig,
			bool,
			func(uint64),
		) syslog.WriteCloser {
			called = true
			return &SleepWriterCloser{metric: func(uint64) {}}
		}

		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
			syslog.WithConstructors(map[string]syslog.WriterConstructor{
				"foo": constructor,
			}),
		)

		binding := syslog.Binding{
			Drain: "foo://",
		}
		_, err := connector.Connect(ctx, binding)
		Expect(err).ToNot(HaveOccurred())
		Expect(called).To(BeTrue())
	})

	It("returns a writer that doesn't block even if the constructor's writer blocks", func() {
		slowConstructor := func(
			*syslog.URLBinding,
			syslog.NetworkTimeoutConfig,
			bool,
			func(uint64),
		) syslog.WriteCloser {
			return &SleepWriterCloser{
				metric:   func(uint64) {},
				duration: time.Hour,
			}
		}

		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
			syslog.WithConstructors(map[string]syslog.WriterConstructor{
				"slow": slowConstructor,
			}),
		)

		binding := syslog.Binding{
			Drain: "slow://",
		}
		writer, err := connector.Connect(ctx, binding)
		Expect(err).ToNot(HaveOccurred())
		err = writer.Write(&loggregator_v2.Envelope{
			SourceId: "test-source-id",
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("returns an error for an unsupported syslog protocol", func() {
		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
		)

		binding := syslog.Binding{
			Drain: "bla://some-domain.tld",
		}
		_, err := connector.Connect(ctx, binding)
		Expect(err).To(MatchError("unsupported protocol"))
	})

	It("returns an error for an inproperly formatted drain", func() {
		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
		)

		binding := syslog.Binding{
			Drain: "://syslog/laksjdflk:asdfdsaf:2232",
		}

		_, err := connector.Connect(ctx, binding)
		Expect(err).To(HaveOccurred())
	})

	It("writes a LGR error for inproperly formatted drains", func() {
		logClient := newSpyLogClient()
		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
			syslog.WithLogClient(logClient, "3"),
		)

		binding := syslog.Binding{
			AppId: "some-app-id",
			Drain: "://syslog/laksjdflk:asdfdsaf:2232",
		}

		_, _ = connector.Connect(ctx, binding)

		Expect(logClient.message()).To(ContainElement("Invalid syslog drain URL: parse failure"))
		Expect(logClient.appID()).To(ContainElement("some-app-id"))
		Expect(logClient.sourceType()).To(HaveKey("LGR"))
	})

	It("emits a metric when sending outbound messages", func() {
		writerConstructor := func(
			_ *syslog.URLBinding,
			_ syslog.NetworkTimeoutConfig,
			_ bool,
			m func(uint64),
		) syslog.WriteCloser {
			return &SleepWriterCloser{metric: m, duration: 0}
		}

		var total uint64
		egressMetric := func(delta uint64) {
			atomic.AddUint64(&total, delta)
		}

		connector := syslog.NewSyslogConnector(
			netConf,
			true,
			spyWaitGroup,
			syslog.WithConstructors(map[string]syslog.WriterConstructor{
				"protocol": writerConstructor,
			}),
			syslog.WithEgressMetrics(map[string]func(uint64){
				"protocol": egressMetric,
			}),
		)

		binding := syslog.Binding{
			Drain: "protocol://",
		}
		writer, err := connector.Connect(ctx, binding)
		Expect(err).ToNot(HaveOccurred())

		for i := 0; i < 500; i++ {
			writer.Write(&loggregator_v2.Envelope{
				SourceId: "test-source-id",
			})
		}

		Eventually(func() uint64 {
			return atomic.LoadUint64(&total)
		}).Should(Equal(uint64(500)))
	})

	Describe("dropping messages", func() {
		var droppingConstructor = func(
			*syslog.URLBinding,
			syslog.NetworkTimeoutConfig,
			bool,
			func(uint64),
		) syslog.WriteCloser {
			return &SleepWriterCloser{
				metric:   func(uint64) {},
				duration: time.Millisecond,
			}
		}

		It("emits a metric on dropped messages", func() {
			var total uint64
			droppedMetric := func(d uint64) { atomic.AddUint64(&total, d) }

			connector := syslog.NewSyslogConnector(
				netConf,
				true,
				spyWaitGroup,
				syslog.WithConstructors(map[string]syslog.WriterConstructor{
					"dropping": droppingConstructor,
				}),
				syslog.WithDroppedMetrics(map[string]func(uint64){
					"dropping": droppedMetric,
				}),
			)

			binding := syslog.Binding{Drain: "dropping://"}

			writer, err := connector.Connect(ctx, binding)
			Expect(err).ToNot(HaveOccurred())

			go func(w syslog.Writer) {
				for {
					w.Write(&loggregator_v2.Envelope{
						SourceId: "test-source-id",
					})
				}
			}(writer)

			Eventually(func() uint64 { return atomic.LoadUint64(&total) }).Should(BeNumerically(">", 10000))
		})

		It("emits a LGR and SYS log to the log client about logs that have been dropped", func() {
			droppedMetric := func(uint64) {}
			binding := syslog.Binding{AppId: "app-id", Drain: "dropping://"}
			logClient := newSpyLogClient()

			connector := syslog.NewSyslogConnector(
				netConf,
				true,
				spyWaitGroup,
				syslog.WithConstructors(map[string]syslog.WriterConstructor{
					"dropping": droppingConstructor,
				}),
				syslog.WithDroppedMetrics(map[string]func(uint64){
					"dropping": droppedMetric,
				}),
				syslog.WithLogClient(logClient, "3"),
			)

			writer, err := connector.Connect(ctx, binding)
			Expect(err).ToNot(HaveOccurred())

			go func(w syslog.Writer) {
				for {
					w.Write(&loggregator_v2.Envelope{
						SourceId: "test-source-id",
					})
				}
			}(writer)

			Eventually(logClient.message).Should(ContainElement(MatchRegexp("\\d messages lost in user provided syslog drain")))
			Eventually(logClient.appID).Should(ContainElement("app-id"))

			Eventually(logClient.sourceType).Should(HaveLen(2))
			Eventually(logClient.sourceType).Should(HaveKey("LGR"))
			Eventually(logClient.sourceType).Should(HaveKey("SYS"))

			Eventually(logClient.sourceInstance).Should(HaveLen(2))
			Eventually(logClient.sourceInstance).Should(HaveKey(""))
			Eventually(logClient.sourceInstance).Should(HaveKey("3"))
		})

		It("does not panic on unknown dropped metrics", func() {
			binding := syslog.Binding{Drain: "dropping://"}

			connector := syslog.NewSyslogConnector(
				netConf,
				true,
				spyWaitGroup,
				syslog.WithConstructors(map[string]syslog.WriterConstructor{
					"dropping": droppingConstructor,
				}),
				syslog.WithDroppedMetrics(map[string]func(uint64){}),
			)

			writer, err := connector.Connect(ctx, binding)
			Expect(err).ToNot(HaveOccurred())

			f := func() {
				for i := 0; i < 50000; i++ {
					writer.Write(&loggregator_v2.Envelope{
						SourceId: "test-source-id",
					})
				}
			}
			Expect(f).ToNot(Panic())
		})
	})
})

type SleepWriterCloser struct {
	duration time.Duration
	io.Closer
	metric func(uint64)
}

func (c *SleepWriterCloser) Write(*loggregator_v2.Envelope) error {
	c.metric(1)
	time.Sleep(c.duration)
	return nil
}
