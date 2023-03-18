package main

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// Set cache age
var cacheAge = time.Hour * 2

//go:embed static/*
var staticContent embed.FS

//go:embed mimetype.json
var mimeTypeContent []byte

type mimeType map[string]string

//go:embed revproxies.json
var revProxiesContent []byte

type revProxyHost struct {
	Scheme     string            `json:"scheme"`
	Host       string            `json:"host"`
	Path       string            `json:"path"`
	ReqHeaders map[string]string `json:"reqHeaders"`
	ResHeaders map[string]string `json:"resHeaders"`
}

func main() {
	checkFlags()

	// Reads the mimetype.json file and converts it to a MimeType type
	var mimeTypes mimeType
	err := json.Unmarshal(mimeTypeContent, &mimeTypes)
	if err != nil {
		log.Fatal(err)
	}

	var revProxyes []revProxyHost
	err = json.Unmarshal(revProxiesContent, &revProxyes)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Reverse proxy handlers
	for _, proxy := range revProxyes {
		proxy := proxy

		log.Println(proxy)
		// Forward requests as a reverse proxy
		apiProxy := httputil.NewSingleHostReverseProxy(&url.URL{
			Scheme: proxy.Scheme,
			Host:   proxy.Host,
		})

		pp := strings.TrimSuffix(proxy.Path, "/")

		// Overwrite Director function and edit request headers
		apiProxy.Director = func(req *http.Request) {
			req.Host = proxy.Host
			req.URL.Scheme = proxy.Scheme
			req.URL.Host = proxy.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, pp)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}

			// Add request headers
			for name, value := range proxy.ReqHeaders {
				req.Header.Set(name, value)
			}
		}

		// Overwrite ModifyResponse function to edit response headers
		apiProxy.ModifyResponse = func(res *http.Response) error {
			// Add response headers
			for name, value := range proxy.ResHeaders {
				res.Header.Set(name, value)
			}

			if 300 <= res.StatusCode && res.StatusCode < 400 {
				loc := res.Header.Get("Location")
				parsedURL, err := url.Parse(loc)
				if err != nil {
					log.Printf("500: %s %s %s", res.Request.RemoteAddr, res.Request.Method, res.Request.RequestURI)
					return err
				}

				// Check if the host is an IP loopback address or "localhost"
				host := parsedURL.Hostname()
				if host == "localhost" || isLoopbackIP(host) {
					// Add the port number if one is not already specified
					if parsedURL.Port() != port {
						parsedURL.Host = host + ":" + port
					}
					if !strings.HasPrefix(parsedURL.Path, proxy.Path) {
						parsedURL.Path = path.Join(proxy.Path, parsedURL.Path)
					}
					if strings.HasSuffix(loc, "/") && !strings.HasSuffix(parsedURL.Path, "/") {
						parsedURL.Path += "/"
					}
				}
				res.Header.Set("Location", parsedURL.String())
			}

			log.Printf("%d: %s %s %s", res.StatusCode, res.Request.RemoteAddr, res.Request.Method, res.Request.RequestURI)
			return nil
		}

		http.Handle(proxy.Path, apiProxy)
	}

	// Root handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get the path to the requested file
		embeddedPath := path.Clean("static/" + r.URL.Path[1:])

		// If the path is a directory, add "/index.html" at the end
		var isDir bool
		if info, err := fs.Stat(staticContent, embeddedPath); err == nil && info.IsDir() {
			isDir = true
			embeddedPath = path.Join(embeddedPath, "index.html")
		}

		// If the file exists, return its contents as a response
		content, err := staticContent.ReadFile(embeddedPath)
		if err != nil {
			// If the file does not exist, return a 404 error
			http.NotFound(w, r)
			log.Printf("404: %s %s %s", r.RemoteAddr, r.Method, r.RequestURI)
			return
		}
		// Redirect if it is a directory but the path does not end with /.
		if isDir && !strings.HasSuffix(r.URL.Path, "/") {
			redirectUrl := r.URL
			redirectUrl.Path += "/"
			http.Redirect(w, r, redirectUrl.String(), http.StatusMovedPermanently)
			log.Printf("301: %s %s %s", r.RemoteAddr, r.Method, r.RequestURI)
			return
		}

		// Set the Content-Type associated with the extension
		ext := path.Ext(embeddedPath)
		contentType, ok := mimeTypes[ext]
		if ok {
			w.Header().Set("Content-Type", contentType)
		}

		// Set up cache control
		if cacheAge == 0 {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(cacheAge.Seconds())))
		}

		// Add Access-Control-Allow-Origin to HTTP header to allow all CORS
		// Get Origin from the request header
		origin := r.Header.Get("Origin")

		// If the request header contains Origin, use its value
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// If Origin is not included, allow all origins
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		// Enable gzip compression
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Write(content)
		} else {
			fmt.Fprint(w, string(content))
		}

		log.Printf("200: %s %s %s", r.RemoteAddr, r.Method, r.RequestURI)
	})

	// Start the server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// Returns true if the given IP address is an IPv4 or IPv6 loopback address
func isLoopbackIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && (parsedIP.IsLoopback() || parsedIP.Equal(net.IPv4(127, 0, 0, 1)) || parsedIP.Equal(net.IPv6loopback))
}

func checkFlags() {
	var versionFlag bool
	flag.BoolVar(&versionFlag, "v", false, "Print the version")
	flag.BoolVar(&versionFlag, "version", false, "Print the version")
	flag.Parse()
	if versionFlag {
		printVersion()
		os.Exit(0)
	}
}

func printVersion() {
	fmt.Println("Version:", Version)
	fmt.Println("Revision:", Revision)
}
