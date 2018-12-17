// Command ghproxy is serves up a github repository.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type githubFiles []struct {
	DownloadURL string  `json:"download_url,omitempty"`
	Name        string  `json:"name,omitempty"`
	Path        string  `json:"path,omitempty"`
	Sha         string  `json:"sha,omitempty"`
	Size        float64 `json:"size,omitempty"`
	Type        string  `json:"type,omitempty"`
	URL         string  `json:"url,omitempty"`
}

func (g githubFiles) downloadURL(path string) string {
	// TODO(tmc): if we traverse this more often consider a map.
	for _, f := range g {
		if f.Path == path {
			return f.DownloadURL
		}
	}
	return ""
}

func (g githubFiles) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	if r.URL.Path == "/" {
		resp, err := http.Get(g.downloadURL("index.html"))
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(rw, "err:", err)
			return
		}
		defer resp.Body.Close()
		rw.Header().Set("Content-Type", "text/html")
		io.Copy(rw, resp.Body)
		return
	}
	// Otherwise send to download link
	for _, f := range g {
		if dlURL := g.downloadURL(r.URL.Path[1:]); dlURL != "" {
			if contentType := contentTypeForPath(f.Path); contentType != "" {
				rw.Header().Set("Content-Type", contentType)
			}
			http.Redirect(rw, r, dlURL, http.StatusFound)
			return
		}
	}
	rw.WriteHeader(http.StatusNotFound)
	fmt.Fprintln(rw, "not found")
}

func contentTypeForPath(p string) string {
	if strings.HasSuffix(p, "whl") {
		return "application/zip"
	}
	if strings.HasSuffix(p, "html") {
		return "text/html"
	}
	return ""
}

func main() {
	flagURL := flag.String("url", "https://api.github.com/repos/user/repo/contents/", "url to proxy")
	flagGithubToken := flag.String("github-token", "", "github token")
	flag.Parse()
	if *flagGithubToken == "" {
		*flagGithubToken = os.Getenv("GITHUB_TOKEN")
	}
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest("GET", *flagURL, nil)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(rw, "err:", err)
			return
		}
		req.SetBasicAuth(*flagGithubToken, "x-oauth-basic")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(rw, "err:", err)
			return
		}
		defer resp.Body.Close()
		files := &githubFiles{}
		err = json.NewDecoder(resp.Body).Decode(files)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(rw, "err:", err)
			return
		}

		files.ServeHTTP(rw, r)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalln(err)
	}
}
