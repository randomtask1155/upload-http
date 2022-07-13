package main_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/randomtask1155/upload-http"
)

var mockHTTPServer *http.Server
var _ = BeforeSuite(func() {
	go func() {
		err := StartHTTPServer()
		if err != nil {
			panic(err)
		}
	}()
	statusUp := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get("http://127.0.0.1:3200/")
		Expect(err).ShouldNot(HaveOccurred())

		if resp.StatusCode == 200 {
			statusUp = true
			break
		}
		time.Sleep(1 * time.Second) // give time for http to start up

	}
	Expect(statusUp).To(Equal(true))
})

var _ = Describe("API Handlers", func() {
	defer GinkgoRecover()

	var hc *http.Client
	hostAddress := "http://127.0.0.1:3200"
	dstFileName := "test.txt"

	BeforeEach(func() {

		InputDir = "/tmp/lv-test-input"
		BinDir = "./"
		LokiAddress = "127.0.0.1:3100"
		IngestCmd = path.Join(BinDir, "ingest")

		os.MkdirAll(InputDir, 0777)
		hc = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})

	Describe("testing handlers", func() {
		Context("when file is uploaded", func() {

			It("upload handler returns 201 response", func() {

				testFile := "/tmp/lv-test-text.txt"
				data, err := os.Create(testFile)
				Expect(err).ShouldNot(HaveOccurred())
				defer data.Close()
				data.Write([]byte("some test data"))
				data.Close()

				file, err := os.Open(testFile)
				Expect(err).ShouldNot(HaveOccurred())
				defer file.Close()

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", dstFileName)
				io.Copy(part, file)
				writer.Close()

				req, err := http.NewRequest("PUT", hostAddress+"/upload", body)
				req.Header.Add("Content-Type", writer.FormDataContentType())
				resp, err := hc.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(201))

			})

			It("A file is created in the input dir", func() {
				_, err := os.Stat(path.Join(InputDir, dstFileName))
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when files are ingested", func() {
			It("emptycache func deletes filecache", func() {
				files, err := ioutil.ReadDir(InputDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(files)).ToNot(Equal(0))
				EmptyInputFolder(InputDir)
				files, err = ioutil.ReadDir(InputDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(files)).To(Equal(0))
			})
		})
	})
})
