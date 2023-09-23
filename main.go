package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/NYTimes/gziphandler"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/svg"
)

var (
	initialProto = getEnv("INITIAL_PROTO", "https")
	initialURI   = getEnv("INITIAL_URI", "monurl.com")
	finalURL     = getEnv("FINAL_URL", "manouvelleurl.com")
	finalProto   = getEnv("FINAL_PROTO", "http")
	JPEGQuality  = getIntEnv("JPEG_QUALITY", 30)
)

type PageCache struct {
	content []byte
	mime    string
}

var cache = make(map[string]PageCache)
var minifier = minify.New()

func init() {
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("text/html", html.Minify)
	minifier.AddFunc("text/javascript", js.Minify)
	minifier.AddFunc("image/svg+xml", svg.Minify)
}

// Fonction pour obtenir une variable d'environnement avec une valeur par défaut
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Fonction pour obtenir une variable d'environnement comme un int avec une valeur par défaut
func getIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func toMD5(input string) string {
	hash := md5.Sum([]byte(input))
	return fmt.Sprintf("%x", hash)
}

func fetchURLBody(url string) ([]byte, string, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", 500, fmt.Errorf("erreur lors de la requête HTTP GET : %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", 500, fmt.Errorf("erreur lors de la lecture du corps de la réponse : %w", err)
	}

	return body, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

func compressJPEG(inputBytes []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(inputBytes))
	if err != nil {
		return nil, err
	}

	var outputBuffer bytes.Buffer
	err = jpeg.Encode(&outputBuffer, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}

	return outputBuffer.Bytes(), nil
}

func optimizeContent(mime, data string) (string, error) {
	switch mime {
	case "text/css", "text/html", "text/javascript", "image/svg+xml":
		return minifier.String(mime, data)
	case "image/jpeg":
		compressed, err := compressJPEG([]byte(data), JPEGQuality)
		if err != nil {
			return "", err
		}
		return string(compressed), nil
	}
	return data, nil
}

func handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	destURL := fmt.Sprintf("%s://%s%s", initialProto, initialURI, r.URL.String())
	signature := toMD5(destURL)

	if page, exists := cache[signature]; exists {
		w.Header().Add("Content-Type", page.mime)
		w.WriteHeader(http.StatusOK)
		w.Write(page.content)
		return
	}

	body, mime, statusCode, err := fetchURLBody(destURL)
	if err != nil {
		log.Printf("Erreur lors de la récupération de l'URL %s : %v", destURL, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	bodyStr := string(body)
	bodyStr = strings.NewReplacer(
		"https://"+initialURI, finalProto+"://"+finalURL,
		"http://"+initialURI, finalProto+"://"+finalURL,
		initialURI, finalURL,
		"https", finalProto,
		"http", finalProto,
	).Replace(bodyStr)

	optimizedBody, err := optimizeContent(mime, bodyStr)
	if err != nil {
		log.Printf("Erreur lors de l'optimisation du contenu : %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", mime)
	w.WriteHeader(statusCode)
	w.Write([]byte(optimizedBody))

	cache[signature] = PageCache{content: []byte(optimizedBody), mime: mime}
}

func main() {
	wrappedHandler := gziphandler.GzipHandler(http.HandlerFunc(handleProxyRequest))
	http.Handle("/", wrappedHandler)
	log.Println("Démarrage du serveur reverse proxy sur :80")
	log.Fatal(http.ListenAndServe(":80", nil))
}
