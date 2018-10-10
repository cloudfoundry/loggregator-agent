package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestForwarderAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForwarderAgent Suite")
}
