package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/julienschmidt/httprouter"
)

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024 // 1GB

var (
	Dir             *string
	Listen          *string
	Addr            *string
	Port            *int
	TablePolynomial *crc32.Table
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

	TablePolynomial = crc32.MakeTable(0xedb88320)
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
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 1GB in size", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, handler, err := r.FormFile("f")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	hash := crc32.New(TablePolynomial)
	if _, err := io.Copy(hash, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	file.Seek(0, io.SeekStart)

	fileName := hex.EncodeToString(hash.Sum(nil)) + filepath.Ext(handler.Filename)
	filePath := path.Join(*Dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(path.Join(*Addr, fileName)))
		return
	}

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
