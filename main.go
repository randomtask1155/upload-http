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
)

var (
	port        = "3200"
	InputDir    = "./ingester/input"
	BinDir      = "./bin"
	LokiAddress = "127.0.0.1:3100"
	IngestCmd   = path.Join(BinDir, "ingest")
	FileCache   []string
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

	FileCache = make([]string, 0)
}

func EmptyCache() {
	for i := range FileCache {
		log.Printf("Deleteing file %s", path.Join(InputDir, FileCache[i]))
		out, err := exec.Command("rm", path.Join(InputDir, FileCache[i])).CombinedOutput()
		if err != nil {
			log.Fatalf("deleting data in input dir failed\n%s\n%s\n", out, err)
			return
		}
	}
	FileCache = make([]string, 0)
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
	FileCache = append(FileCache, fh.Filename)
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

	out, err := exec.Command(IngestCmd, LokiAddress, req.Filter).CombinedOutput()
	if err != nil {
		log.Printf("running ingest failed\n%s\n%s\n", out, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println(out)
	EmptyCache()
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	usage := `
<html>
<header>
<title>log-visualizer-proxy</title>
</header>
<body>

uploading files:
curl -X PUT -H "Content-Type:multipart/form-data" http://ipaddress:3200/upload -F "file=log-file.tgz"

ingest uploaded files:
curl -X POST -H "Content-Type:application/json" http://ipaddress:3200/ingest -d '{ filter: "/gorouter/access.*|/gorouter/gorouter.stdout.*|/bosh-dns/bosh_dns.stdout.*"}'

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
