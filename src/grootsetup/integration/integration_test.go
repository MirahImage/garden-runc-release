package integration_test

import (
	. "grootsetup/integration"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("grootsetup integration", func() {
	var storeMountpoint string

	BeforeEach(func() {
		var err error

		storeMountpoint, err = ioutil.TempDir("", "test-store-mountpoint")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storeMountpoint)).To(Succeed())
	})

	It("creates the grootfs/store directory inside the store mountpoint", func() {

	})
})
