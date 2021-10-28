package openliberty_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnit(t *testing.T) {
	suite := spec.New("openliberty", spec.Report(report.Terminal{}))
	// suite("Build", testBuild)
	suite("Detect", testDetect)
	// suite("Distribution", testDistribution)
	suite.Run(t)
}
