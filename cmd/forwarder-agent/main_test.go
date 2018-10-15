package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"time"

	"code.cloudfoundry.org/loggregator-agent/internal/testhelper"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var (
		cupsProvider *httptest.Server
	)

	BeforeEach(func() {
		cupsProvider = httptest.NewServer(
			&fakeCC{
				results: results{
					"9be15160-4845-4f05-b089-40e827ba61f1": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"syslog://1.1.1.1",
						},
					},
					"9be15160-4845-4f05-b089-40e827ba61f2": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-syslog://1.1.1.1",
						},
					},
					"9be15160-4845-4f05-b089-40e827ba61f13": appBindings{
						Hostname: "org.space.name",
						Drains: []string{
							"v3-syslog://1.1.1.2",
						},
					},
				},
			},
		)
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	It("has a health endpoint", func() {
		session := startForwarderAgent(
			fmt.Sprintf("API_CA_FILE_PATH=%s", testhelper.Cert("capi-ca.crt")),
			fmt.Sprintf("API_CERT_FILE_PATH=%s", testhelper.Cert("forwarder.crt")),
			fmt.Sprintf("API_COMMON_NAME=%s", "capiCA"),
			fmt.Sprintf("API_KEY_FILE_PATH=%s", testhelper.Cert("forwarder.key")),
			"API_POLLING_INTERVAL=10ms",
			"DEBUG_PORT=7392",
			fmt.Sprintf("API_URL=%s", cupsProvider.URL),
		)
		defer session.Kill()

		Eventually(func() int {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return -1
			}

			return resp.StatusCode
		}).Should(Equal(http.StatusOK))

		f := func() string {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			return string(body)
		}
		Eventually(f, 3*time.Second, 500*time.Millisecond).Should(ContainSubstring(`"drainCount": 2`))
	})
})

func startForwarderAgent(envs ...string) *gexec.Session {
	path, err := gexec.Build("code.cloudfoundry.org/loggregator-agent/cmd/forwarder-agent")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(path)
	cmd.Env = envs
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		panic(err)
	}

	return session
}

type results map[string]appBindings

type appBindings struct {
	Drains   []string `json:"drains"`
	Hostname string   `json:"hostname"`
}

type fakeCC struct {
	count           int
	called          bool
	withEmptyResult bool
	results         results
}

func (f *fakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/internal/v4/syslog_drain_urls" {
		w.WriteHeader(500)
		return
	}

	if r.URL.Query().Get("batch_size") != "1000" {
		w.WriteHeader(500)
		return
	}

	f.serveWithResults(w, r)
}

func (f *fakeCC) serveWithResults(w http.ResponseWriter, r *http.Request) {
	resultData, err := json.Marshal(struct {
		Results results `json:"results"`
	}{
		Results: f.results,
	})
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if f.count > 0 {
		resultData = []byte(`{"results": {}}`)
	}

	w.Write(resultData)
	if f.withEmptyResult {
		f.count++
	}
}
