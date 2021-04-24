package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/julienschmidt/httprouter"
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

func HealthHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}

func IndexHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("curl -F'f=@file' " + *Addr))
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	r.ParseMultipartForm(0)
	defer r.MultipartForm.RemoveAll()

	file, handler, err := r.FormFile("f")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	defer file.Close()

	fileName := ksuid.New().String() + filepath.Ext(handler.Filename)
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

func GetFileHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	filePath := path.Join(*Dir, ps.ByName("filename"))
	http.ServeFile(w, r, filePath)
}

func main() {
	router := httprouter.New()
	router.GET("/", IndexHandler)
	router.GET("/:filename", GetFileHandler)
	router.POST("/", UploadFileHandler)

	srv := &http.Server{
		Handler:      router,
		Addr:         *Listen + ":" + fmt.Sprint(*Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Server started listening on %s", srv.Addr)

	log.Fatal(srv.ListenAndServe())
}
