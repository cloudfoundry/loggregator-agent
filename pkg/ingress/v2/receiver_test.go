package v2_test

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	ingress "code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Receiver", func() {
	var (
		rx        *ingress.Receiver
		spySetter *SpySetter
	)

	BeforeEach(func() {
		spySetter = NewSpySetter()
		rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, &testhelper.SpyMetric{})
	})

	Describe("Sender()", func() {
		var (
			spySender *SpySender
		)

		BeforeEach(func() {
			spySender = NewSpySender()
		})

		It("calls set on the data setter with the data", func() {
			eActual := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			eExpected := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			spySender.recvResponses <- SenderRecvResponse{
				envelope: eActual,
			}
			spySender.recvResponses <- SenderRecvResponse{
				envelope: eActual,
			}
			spySender.recvResponses <- SenderRecvResponse{
				err: io.EOF,
			}

			rx.Sender(spySender)

			Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))
			Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))
		})

		It("returns an error when receive fails", func() {
			spySender.recvResponses <- SenderRecvResponse{
				err: errors.New("error occurred"),
			}

			err := rx.Sender(spySender)

			Expect(err).To(HaveOccurred())
		})

		It("increments the ingress metric", func() {
			ingressMetric := &testhelper.SpyMetric{}
			rx = ingress.NewReceiver(spySetter, ingressMetric, &testhelper.SpyMetric{})

			e := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			spySender.recvResponses <- SenderRecvResponse{
				envelope: e,
			}
			spySender.recvResponses <- SenderRecvResponse{
				envelope: e,
			}
			spySender.recvResponses <- SenderRecvResponse{
				err: io.EOF,
			}

			rx.Sender(spySender)

			Expect(ingressMetric.Value()).To(BeNumerically("==", 2))
		})

		Context("when source ID is not set", func() {
			It("sets the source ID with the origin tag value", func() {
				originMappingMetric := &testhelper.SpyMetric{}
				rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

				eActual := &loggregator_v2.Envelope{
					Tags: map[string]string{"origin": "some-origin"},
				}

				eExpected := &loggregator_v2.Envelope{
					SourceId: "some-origin",
					Tags:     map[string]string{"origin": "some-origin"},
				}

				spySender.recvResponses <- SenderRecvResponse{
					envelope: eActual,
				}
				spySender.recvResponses <- SenderRecvResponse{
					err: io.EOF,
				}

				rx.Sender(spySender)

				Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))

				Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
			})

			Context("when the origin tag is not set", func() {
				It("sets the source ID with the origin deprecated tag value", func() {
					originMappingMetric := &testhelper.SpyMetric{}
					rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

					eActual := &loggregator_v2.Envelope{
						DeprecatedTags: map[string]*loggregator_v2.Value{
							"origin": {
								Data: &loggregator_v2.Value_Text{
									Text: "deprecated-origin",
								},
							},
						},
					}

					eExpected := &loggregator_v2.Envelope{
						SourceId: "deprecated-origin",
						DeprecatedTags: map[string]*loggregator_v2.Value{
							"origin": {
								Data: &loggregator_v2.Value_Text{
									Text: "deprecated-origin",
								},
							},
						},
					}

					spySender.recvResponses <- SenderRecvResponse{
						envelope: eActual,
					}
					spySender.recvResponses <- SenderRecvResponse{
						err: io.EOF,
					}

					rx.Sender(spySender)

					Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))
					Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
				})
			})

			Context("no origin or source id is set", func() {
				It("sets the source ID with the origin deprecated tag value", func() {
					originMappingMetric := &testhelper.SpyMetric{}
					rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

					eActual := &loggregator_v2.Envelope{}

					spySender.recvResponses <- SenderRecvResponse{
						envelope: eActual,
					}
					spySender.recvResponses <- SenderRecvResponse{
						err: io.EOF,
					}

					rx.Sender(spySender)

					Expect(spySetter.envelopes).To(Receive(Equal(eActual)))

					Expect(originMappingMetric.Value()).To(BeNumerically("==", 0))
				})
			})
		})
	})

	Describe("BatchSender()", func() {
		var (
			spyBatchSender *SpyBatchSender
		)

		BeforeEach(func() {
			spyBatchSender = NewSpyBatchSender()
		})

		It("calls set on the datasetting with all the envelopes", func() {
			e := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				envelopes: []*loggregator_v2.Envelope{e, e, e, e, e},
			}
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: io.EOF,
			}

			rx.BatchSender(spyBatchSender)

			Expect(spySetter.envelopes).Should(HaveLen(5))
		})

		It("returns an error when receive fails", func() {
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: errors.New("error occurred"),
			}

			err := rx.BatchSender(spyBatchSender)

			Expect(err).To(HaveOccurred())
		})

		It("increments the ingress metric", func() {
			ingressMetric := &testhelper.SpyMetric{}
			rx = ingress.NewReceiver(spySetter, ingressMetric, &testhelper.SpyMetric{})

			e := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				envelopes: []*loggregator_v2.Envelope{e, e, e, e, e},
			}
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: io.EOF,
			}

			rx.BatchSender(spyBatchSender)

			Expect(spySetter.envelopes).Should(HaveLen(5))
			Expect(ingressMetric.Value()).To(BeNumerically("==", 5))
		})

		It("sets the source ID with the origin value when missing source ID", func() {
			originMappingMetric := &testhelper.SpyMetric{}
			rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

			e1Actual := &loggregator_v2.Envelope{
				Tags: map[string]string{"origin": "some-origin"},
			}

			e1Expected := &loggregator_v2.Envelope{
				SourceId: "some-origin",
				Tags:     map[string]string{"origin": "some-origin"},
			}

			e2Actual := &loggregator_v2.Envelope{
				SourceId: "some-id-2",
				Tags:     map[string]string{"origin": "some-origin"},
			}

			e2Expected := &loggregator_v2.Envelope{
				SourceId: "some-id-2",
				Tags:     map[string]string{"origin": "some-origin"},
			}

			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				envelopes: []*loggregator_v2.Envelope{e1Actual, e2Actual},
			}
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: io.EOF,
			}

			rx.BatchSender(spyBatchSender)

			Expect(spySetter.envelopes).Should(Receive(Equal(e1Expected)))
			Expect(spySetter.envelopes).Should(Receive(Equal(e2Expected)))

			Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
		})

		Context("when the origin tag is not set", func() {
			It("sets the source ID with the origin deprecated tag value", func() {
				originMappingMetric := &testhelper.SpyMetric{}
				rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

				eActual := &loggregator_v2.Envelope{
					DeprecatedTags: map[string]*loggregator_v2.Value{
						"origin": {
							Data: &loggregator_v2.Value_Text{
								Text: "deprecated-origin",
							},
						},
					},
				}

				eExpected := &loggregator_v2.Envelope{
					SourceId: "deprecated-origin",
					DeprecatedTags: map[string]*loggregator_v2.Value{
						"origin": {
							Data: &loggregator_v2.Value_Text{
								Text: "deprecated-origin",
							},
						},
					},
				}

				spyBatchSender.recvResponses <- BatchSenderRecvResponse{
					envelopes: []*loggregator_v2.Envelope{eActual},
				}
				spyBatchSender.recvResponses <- BatchSenderRecvResponse{
					err: io.EOF,
				}

				rx.BatchSender(spyBatchSender)

				Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))

				Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
			})
		})
	})

	Describe("Send()", func() {
		It("calls set on the setter with the given envelopes", func() {
			e1Actual := &loggregator_v2.Envelope{
				SourceId: "some-id-1",
				Tags:     map[string]string{"origin": "my-origin"},
			}
			e2Actual := &loggregator_v2.Envelope{
				SourceId: "some-id-2",
				Tags:     map[string]string{"origin": "my-origin"},
			}
			e1Expected := &loggregator_v2.Envelope{
				SourceId: "some-id-1",
				Tags:     map[string]string{"origin": "my-origin"},
			}
			e2Expected := &loggregator_v2.Envelope{
				SourceId: "some-id-2",
				Tags:     map[string]string{"origin": "my-origin"},
			}

			rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{e1Actual, e2Actual},
			})

			Expect(spySetter.envelopes).To(Receive(Equal(e1Expected)))
			Expect(spySetter.envelopes).To(Receive(Equal(e2Expected)))
		})

		It("increments the ingress metric", func() {
			ingressMetric := &testhelper.SpyMetric{}
			rx = ingress.NewReceiver(spySetter, ingressMetric, &testhelper.SpyMetric{})

			e := &loggregator_v2.Envelope{
				SourceId: "some-id",
			}

			rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{e},
			})

			Expect(ingressMetric.Value()).To(BeNumerically("==", 1))
		})

		It("increments the origin_mappings metric", func() {
			originMappingMetric := &testhelper.SpyMetric{}
			rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

			e := &loggregator_v2.Envelope{
				Tags: map[string]string{"origin": "my-origin"},
			}

			rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{e},
			})

			Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
		})

		Context("when source ID is not set", func() {
			It("sets source ID with origin tag", func() {
				eActual := &loggregator_v2.Envelope{
					Tags: map[string]string{"origin": "some-origin-1"},
				}

				eExpected := &loggregator_v2.Envelope{
					SourceId: "some-origin-1",
					Tags:     map[string]string{"origin": "some-origin-1"},
				}

				rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{eActual},
				})

				Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))
			})

			Context("when the origin tag is not set", func() {
				It("sets the source ID with the origin deprecated tag value", func() {
					originMappingMetric := &testhelper.SpyMetric{}
					rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

					eActual := &loggregator_v2.Envelope{
						DeprecatedTags: map[string]*loggregator_v2.Value{
							"origin": {
								Data: &loggregator_v2.Value_Text{
									Text: "deprecated-origin",
								},
							},
						},
					}

					eExpected := &loggregator_v2.Envelope{
						SourceId: "deprecated-origin",
						DeprecatedTags: map[string]*loggregator_v2.Value{
							"origin": {
								Data: &loggregator_v2.Value_Text{
									Text: "deprecated-origin",
								},
							},
						},
					}

					rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
						Batch: []*loggregator_v2.Envelope{eActual},
					})

					Expect(spySetter.envelopes).To(Receive(Equal(eExpected)))
					Expect(originMappingMetric.Value()).To(BeNumerically("==", 1))
				})
			})

			Context("no origin or source id is set", func() {
				It("sets the source ID with the origin deprecated tag value", func() {
					originMappingMetric := &testhelper.SpyMetric{}
					rx = ingress.NewReceiver(spySetter, &testhelper.SpyMetric{}, originMappingMetric)

					eActual := &loggregator_v2.Envelope{}

					rx.Send(context.Background(), &loggregator_v2.EnvelopeBatch{
						Batch: []*loggregator_v2.Envelope{eActual},
					})

					Expect(spySetter.envelopes).To(Receive(Equal(eActual)))

					Expect(originMappingMetric.Value()).To(BeNumerically("==", 0))
				})
			})
		})
	})
})

type SenderRecvResponse struct {
	envelope *loggregator_v2.Envelope
	err      error
}

type BatchSenderRecvResponse struct {
	envelopes []*loggregator_v2.Envelope
	err       error
}

type SpySender struct {
	loggregator_v2.Ingress_SenderServer
	recvResponses chan SenderRecvResponse
}

func NewSpySender() *SpySender {
	return &SpySender{
		recvResponses: make(chan SenderRecvResponse, 100),
	}
}

func (s *SpySender) Recv() (*loggregator_v2.Envelope, error) {
	resp := <-s.recvResponses

	return resp.envelope, resp.err
}

type SpyBatchSender struct {
	loggregator_v2.Ingress_BatchSenderServer
	recvResponses chan BatchSenderRecvResponse
}

func NewSpyBatchSender() *SpyBatchSender {
	return &SpyBatchSender{
		recvResponses: make(chan BatchSenderRecvResponse, 100),
	}
}

func (s *SpyBatchSender) Recv() (*loggregator_v2.EnvelopeBatch, error) {
	resp := <-s.recvResponses

	return &loggregator_v2.EnvelopeBatch{Batch: resp.envelopes}, resp.err
}

type SpySetter struct {
	envelopes chan *loggregator_v2.Envelope
}

func NewSpySetter() *SpySetter {
	return &SpySetter{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
	}
}

func (s *SpySetter) Set(e *loggregator_v2.Envelope) {
	s.envelopes <- e
}
