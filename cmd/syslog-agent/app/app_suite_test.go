package app_test

import (
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"
)

func TestApp(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "App Suite")
}
