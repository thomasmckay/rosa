package whoami_test

import (
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/decorators"
	. "github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
	"github.com/openshift/rosa/cmd/whoami"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

const (
	version string = "4.10.1"
	state   string = "running"
)

var (
	now = time.Now()
)

var _ = BeforeSuite(func() {
})

var _ = Describe("Cluster description", Ordered, func() {

	Context("when displaying clusters with output json", func() {

		DescribeTable("When displaying clusters with output json",
			printJson,
			Entry("Prints empty when all values are empty", "test1"),
		)
	})
})

func printJson(name string) {

	// Start our recorder
	vcr, err := recorder.New("fixtures/whoami")
	Expect(err).To(BeNil())

	defer vcr.Stop() // Make sure recorder is stopped once done with it

	Expect(vcr.Mode()).To(Equal(recorder.ModeRecordOnce))

	cmd := whoami.Cmd

	r, w, _ := os.Pipe()
	tmp := os.Stdout
	defer func() {
		os.Stdout = tmp
	}()
	os.Stdout = w
	go func() {
		cmd.Run(nil, nil)
		w.Close()
	}()
	stdout, _ := io.ReadAll(r)
	Expect(string(stdout)).To(Equal("" +
		"AWS ARN:                      arn:aws:iam::765374464689:user/tomckay@redhat.com\n" +
		"AWS Account ID:               765374464689\n" +
		"AWS Default Region:           us-east-1\n" +
		"OCM API:                      http://localhost:9000\n" +
		"OCM Account Email:            tomckay@redhat.com\n" +
		"OCM Account ID:               2OYtkDODD8hPWF6gnRECpMvzxGg\n" +
		"OCM Account Name:             Thomas McKay\n" +
		"OCM Account Username:         tomckay.openshift\n" +
		"OCM Organization External ID: 12541229\n" +
		"OCM Organization ID:          1jIHnIbrnLH9kQD57W0BuPm78f1\n" +
		"OCM Organization Name:        Red Hat : Service Delivery : SDA/B"))
}
