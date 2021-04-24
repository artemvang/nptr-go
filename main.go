package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/segmentio/ksuid"
)

var (
	Dir    *string
	Listen *string
	Addr   *string
	Port   *int
)

func init() {
	Dir = flag.String("dir", "directory", "A directory to store uploaded files")
	Port = flag.Int("port", 8000, "Listen port")
	Listen = flag.String("listen", "127.0.0.1", "Listen host")
	Addr = flag.String("addr", "http://127.0.0.1:8000", "Service address")

	flag.Parse()

	if _, err := os.Stat(*Dir); os.IsNotExist(err) {
		log.Fatalf("Uploads directory does not exist | %s", *Dir)
	}
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("curl nptr"))
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("f")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	defer file.Close()

	fileName := ksuid.New().String()

	f, err := os.OpenFile(path.Join(*Dir, fileName), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(path.Join(*Addr, fileName)))

	log.Printf("New file uploaded | %s", fileName)
}

func GetFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filePath := path.Join(*Dir, vars["filename"])
	http.ServeFile(w, r, filePath)
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/health", HealthHandler)
	r.HandleFunc("/", IndexHandler).Methods("GET")
	r.HandleFunc("/", UploadFileHandler).Methods("POST")
	r.HandleFunc("/{filename}", GetFileHandler)

	srv := &http.Server{
		Handler:      r,
		Addr:         *Listen + ":" + fmt.Sprint(*Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Server started listening on %s", srv.Addr)

	log.Fatal(srv.ListenAndServe())
}