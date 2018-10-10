package main_test

import (
	"net/http"
	"os/exec"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	It("has a health endpoint", func() {
		session := startForwarderAgent("DEBUG_PORT=7392")
		defer session.Kill()

		Eventually(func() int {
			resp, err := http.Get("http://127.0.0.1:7392/debug/vars")
			if err != nil {
				return -1
			}

			return resp.StatusCode
		}).Should(Equal(http.StatusOK))
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
