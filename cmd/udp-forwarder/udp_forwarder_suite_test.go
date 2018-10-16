package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUdpForwarder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UdpForwarder Suite")
}
