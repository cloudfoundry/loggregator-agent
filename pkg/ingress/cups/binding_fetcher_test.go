package cups_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BindingFetcher", func() {
	var (
		getter  *SpyGetter
		fetcher *cups.BindingFetcher
	)

	BeforeEach(func() {
		getter = &SpyGetter{}
		fetcher = cups.NewBindingFetcher(getter)
	})

	Context("when the status code is 200 and the body is valid json", func() {
		BeforeEach(func() {
			getter.getResponses = []*http.Response{
				&http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(strings.NewReader(`
							{
							  "results": {
								"9be15160-4845-4f05-b089-40e827ba61f1": {
								  "drains": [
									"syslog://some.url",
									"syslog://some.other.url",
									"syslog-v3://v3.other.url",
									"https-v3://v3.other.url"
								  ],
								  "hostname": "org.space.logspinner"
								}
							  },
							  "next_id": 50
							}
					`)),
				},

				&http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(strings.NewReader(`{ "results": { }, "next_id": null }`)),
				},
			}
		})

		It("returns v3 bindings", func() {
			bindings, err := fetcher.FetchBindings()
			Expect(err).ToNot(HaveOccurred())
			Expect(bindings).To(HaveLen(2))

			appID := "9be15160-4845-4f05-b089-40e827ba61f1"
			Expect(bindings).To(ConsistOf(
				syslog.Binding{
					AppId:    appID,
					Hostname: "org.space.logspinner",
					Drain:    "syslog://v3.other.url",
				},
				syslog.Binding{
					AppId:    appID,
					Hostname: "org.space.logspinner",
					Drain:    "https://v3.other.url",
				}))
		})

		It("fetches all the pages", func() {
			fetcher.FetchBindings()
			Expect(getter.getCalled).To(Equal(2))
			Expect(getter.getNextID[0]).To(Equal(0))
			Expect(getter.getNextID[1]).To(Equal(50))
		})

		Context("when the status code is 200 and the body is invalid json", func() {
			BeforeEach(func() {
				getter.getResponses = []*http.Response{
					&http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(strings.NewReader("invalid")),
					},
				}
			})

			It("returns an error", func() {
				_, err := fetcher.FetchBindings()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the status code is not 200", func() {
			BeforeEach(func() {
				getter.getResponses = []*http.Response{{StatusCode: http.StatusBadRequest}}
			})

			It("returns an error", func() {
				_, err := fetcher.FetchBindings()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	It("returns an error when the getter fails", func() {
		getter.getResponses = []*http.Response{{StatusCode: 500}}
		getter.getError = errors.New("some-error")

		_, err := fetcher.FetchBindings()

		Expect(err).To(HaveOccurred())
	})
})

type SpyGetter struct {
	currentResponse int
	getCalled       int
	getNextID       []int
	getResponses    []*http.Response
	getError        error
}

func (s *SpyGetter) Get(nextID int) (*http.Response, error) {
	s.getCalled++
	s.getNextID = append(s.getNextID, nextID)
	resp := s.getResponses[s.currentResponse]
	s.currentResponse++

	if s.getError != nil {
		return nil, s.getError
	}

	return resp, nil
}
