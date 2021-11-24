package main

import (
	"encoding/hex"
	"flag"
	"hash/crc32"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/valyala/fasthttp"
)

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024 // 1GB

var (
	Dir             *string
	Socket          *string
	AddrURL         *url.URL
	TablePolynomial *crc32.Table
	IndexPage       string
)

func init() {
	var err error
	Dir = flag.String("dir", "directory", "A directory to store uploaded files")
	Socket = flag.String("socket", "/run/nptr.sock", "Listen socket")

	addr := flag.String("addr", "http://127.0.0.1:8000", "Service address")

	flag.Parse()

	if _, err := os.Stat(*Dir); os.IsNotExist(err) {
		log.Fatalf("Uploads directory does not exist | %s", *Dir)
	}

	AddrURL, err = url.Parse(*addr)
	if err != nil {
		log.Fatalf("Provided invalid url | %s", *addr)
	}

	IndexPage = "curl -F'f=@f' " + *addr
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

	u, _ := url.Parse(AddrURL.String())
	u.Path = path.Join(u.Path, fileName)

	if _, err := os.Stat(filePath); os.IsExist(err) {
		ctx.SetBodyString(u.String())
		return
	}

	if f, err = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0666); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	defer f.Close()
	io.Copy(f, file)
	ctx.SetBodyString(u.String())

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
		case "/health":
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

	srv := &fasthttp.Server{
		Name:               "nptr-go",
		Handler:            requestHandler,
		MaxRequestBodySize: MAX_UPLOAD_SIZE,
	}

	err := syscall.Unlink(*Socket)
	if err != nil {
		log.Print("Unlink()", err)
	}

	ln, err := net.Listen("unix", *Socket)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	if err = os.Chmod(*Socket, 0775); err != nil {
		log.Fatal(err)
	}

	log.Printf("Server started listening on %s", *Socket)

	if err := srv.Serve(ln); err != nil {
		log.Panic(err)
	}
}
