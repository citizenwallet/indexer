package wallet

import (
	"compress/gzip"
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

type Server struct {
	path string
	uiFS fs.FS
}

func NewServer(path string) *Server {
	uiFS, err := fs.Sub(app.UI, "_ui/wallet")
	if err != nil {
		log.Fatal("failed to find the ui files", err)
	}

	return &Server{
		path,
		uiFS,
	}
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleStatic)

	return http.ListenAndServe(fmt.Sprintf(":%v", port), mux)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// make sure that we format the path correctly to host a single page app
	path := cleanPathSPA(r.URL.Path)

	// check if the file exists in our virtual file system
	file, err := s.uiFS.Open(path)
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

	// set the content type
	contentType := mime.TypeByExtension(filepath.Ext(path))
	w.Header().Set("Content-Type", contentType)

	// set the cache control header for static files
	if strings.HasPrefix(path, "static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	// set the content length header
	stat, err := file.Stat()
	if err == nil && stat.Size() > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	}

	// gzip compression
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")

		gz := gzip.NewWriter(w)
		defer gz.Close()

		_, err = io.Copy(gz, file)
		if err != nil {
			log.Println("failed to gzip file", path, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		return
	}

	// copy the file to the response writer
	n, err := io.Copy(w, file)
	if err != nil {
		log.Println("failed to copy file", path, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Println("file", path, "copied", n, "bytes")
}

// cleanPathSPA is a helper function to clean the path for SPA
func cleanPathSPA(path string) string {
	path = filepath.Clean(path)
	if path == "/" {
		path = "index.html"
	}
	path = strings.TrimPrefix(path, "/")

	// Check if the path has an extension
	if filepath.Ext(path) == "" {
		// If not, add .html
		path = path + ".html"
	}

	return path
}
