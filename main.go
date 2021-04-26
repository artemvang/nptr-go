package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"

	"github.com/valyala/fasthttp"
)

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024 // 1GB

var (
	Dir             *string
	Listen          *string
	Addr            *string
	Port            *int
	TablePolynomial *crc32.Table
	IndexPage       string
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

	IndexPage = "curl -F'f=@f' " + *Addr
	TablePolynomial = crc32.MakeTable(0xedb88320)
}

func IndexHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetBodyString(IndexPage)
}

func UploadFileHandler(ctx *fasthttp.RequestCtx) {
	var (
		file multipart.File
		f    *os.File
	)

	fileHeader, err := ctx.FormFile("f")
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	if file, err = fileHeader.Open(); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}
	defer file.Close()

	hash := crc32.New(TablePolynomial)
	if _, err := io.Copy(hash, file); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	file.Seek(0, 0)

	fileName := hex.EncodeToString(hash.Sum(nil)) + filepath.Ext(fileHeader.Filename)
	filePath := path.Join(*Dir, fileName)

	if _, err := os.Stat(filePath); os.IsExist(err) {
		ctx.SetBodyString(path.Join(*Addr, fileName))
		return
	}

	if f, err = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0666); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	defer f.Close()
	io.Copy(f, file)
	ctx.SetBodyString(path.Join(*Addr, fileName))

	logger := ctx.Logger()
	logger.Printf("new upload - %s", fileName)
}

func main() {
	fs := &fasthttp.FS{
		Root:               *Dir,
		IndexNames:         []string{},
		GenerateIndexPages: false,
		Compress:           false,
		AcceptByteRange:    true,
	}

	fsHandler := fs.NewRequestHandler()

	requestHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/status":
			ctx.SetBodyString("ok")
		case "/":
			switch method := string(ctx.Method()); method {
			case "GET":
				IndexHandler(ctx)
			case "POST":
				UploadFileHandler(ctx)
			}
		default:
			fsHandler(ctx)
		}
	}

	listenFull := fmt.Sprintf("%s:%d", *Listen, *Port)
	srv := &fasthttp.Server{
		Name:               "nptr-go",
		Handler:            requestHandler,
		MaxRequestBodySize: MAX_UPLOAD_SIZE,
	}

	log.Printf("Server started listening on %s", listenFull)

	if err := srv.ListenAndServe(listenFull); err != nil {
		log.Panic(err)
	}
}
