package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPromScraper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PromScraper Suite")
}
