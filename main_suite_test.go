package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJohari(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "log visualizer ingest Suite")
}
