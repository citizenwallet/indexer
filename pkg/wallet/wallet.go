package wallet

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	app "github.com/citizenwallet/indexer"
)

var uiFS fs.FS

func init() {
	var err error
	uiFS, err = fs.Sub(app.UI, "_ui/wallet")
	if err != nil {
		log.Fatal("failed to get ui fs", err)
	}
}

type Server struct {
	path string
}

func NewServer(path string) *Server {
	return &Server{
		path,
	}
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleStatic)

	return http.ListenAndServe(fmt.Sprintf(":%v", port), mux)
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Clean(r.URL.Path)
	if path == "/" { // Add other paths that you route on the UI side here
		path = "index.html"
	}
	path = strings.TrimPrefix(path, "/")

	// Check if the path has an extension
	if filepath.Ext(path) == "" {
		// If not, add .html
		path = path + ".html"
	}

	file, err := uiFS.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("file", path, "not found:", err)
			http.NotFound(w, r)
			return
		}
		log.Println("file", path, "cannot be read:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))
	w.Header().Set("Content-Type", contentType)
	if strings.HasPrefix(path, "static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}
	stat, err := file.Stat()
	if err == nil && stat.Size() > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	}

	n, _ := io.Copy(w, file)
	log.Println("file", path, "copied", n, "bytes")
}
