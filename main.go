package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"
)

var (
	port        = "3200"
	InputDir    = "./ingester/input"
	BinDir      = "./"
	LokiAddress = "127.0.0.1:3100"
	IngestCmd   = path.Join(BinDir, "bin/ingest")
	IngestLock  chan struct{}
	Version     string
)

type IngestRequest struct {
	Filter string `json:"filter"`
}

func init() {
	p := os.Getenv("PORT")
	if p != "" {
		port = p
	}

	id := os.Getenv("INPUT_DIR")
	if id != "" {
		InputDir = id
	}

	bd := os.Getenv("BIN_DIR")
	if id != "" {
		BinDir = bd
	}

	la := os.Getenv("LOKI_ADDRESS")
	if la != "" {
		LokiAddress = la
	}

	IngestLock = make(chan struct{}, 1)

	if err := os.Chdir(BinDir); err != nil {
		panic(err)
	}
}

func EmptyInputFolder(d string) {
	files, err := ioutil.ReadDir(d)
	if err != nil {
		log.Fatalf("deleting data in input dir failed\n%s", err)
	}
	for i := range files {
		filename := path.Join(d, files[i].Name())
		log.Printf("Deleteing %s", filename)
		out, err := exec.Command("rm", "-rf", filename).CombinedOutput()
		if err != nil {
			log.Fatalf("deleting data in input dir failed\n%s\n%s\n", out, err)
			return
		}
	}
}

func LaunchIngester(req IngestRequest, ch chan struct{}, d string) {
	out, err := exec.Command(IngestCmd, LokiAddress, req.Filter).CombinedOutput()
	if err != nil {
		log.Printf("running ingest failed\n%s\n%s\n", out, err)
		return
	}
	log.Println(fmt.Sprintf("%s", out))
	EmptyInputFolder(d)
	<-ch
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, fh, err := r.FormFile("file")
	if err != nil {
		log.Printf("parsing form: %s\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	dst, err := os.Create(fmt.Sprintf("%s/%s", InputDir, fh.Filename))
	if err != nil {
		log.Printf("creating destination file: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("copy to destination file: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func ingestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := IngestRequest{}
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("failed u unmarshal request body: %s\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// make sure only one ingest is running at any given time
	select {
	case IngestLock <- struct{}{}:
		go LaunchIngester(req, IngestLock, InputDir)
		output := `
#################################################
ingest is runnig and you can check for errors on 
the grafana-loki vm by running the following 
sequence
#################################################

grafana-loki/a5170227-c641-4be6-9afb-78289ad29eeb:/var/vcap/store# docker ps -a | egrep ingester
1838bb55d8cf   ingester_log-processor   "/log-processor.sh /…"   22 minutes ago      Exited (0) 21 minutes ago                              ingester_log-processor_run_ade386723116


grafana-loki/a5170227-c641-4be6-9afb-78289ad29eeb:/var/vcap/store# docker logs 1838bb55d8cf | tail


`
		w.Write([]byte(output))
	case <-time.After(time.Second):
		http.Error(w, "ingest is already running", http.StatusTooManyRequests)
		return
	}

}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	usage := `
<html>
<header>
<title>log-visualizer-proxy</title>
</header>
<body>

<p>Version ` + Version + `</p>
<p>
uploading files:<br>
curl -X PUT -H "Content-Type:multipart/form-data" http://ipaddress:3200/upload -F "file=@log-file.tgz"

<br><br>

ingest uploaded files:<br>
curl -X POST -H "Content-Type:application/json" http://ipaddress:3200/ingest -d '{ "filter": "/gorouter/access.*|/gorouter/gorouter.stdout.*|/bosh-dns/bosh_dns.stdout.*"}'
<br>
</p>
</body>
</html>
`
	w.Write([]byte(usage))
}

func StartHTTPServer() error {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/upload", UploadHandler)
	http.HandleFunc("/ingest", ingestHandler)
	return http.ListenAndServe(":"+port, nil)
}

func main() {
	if err := StartHTTPServer(); err != nil {
		log.Fatalln(err)
	}
}
