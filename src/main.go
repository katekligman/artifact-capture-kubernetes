package main

import (
  "os/exec"
  "crypto/md5"
  "encoding/base64"
  "fmt"
  "log"
  "net/http"
  "time"
  "io/ioutil"
  "golang.org/x/oauth2/google"
  "google.golang.org/cloud"
  "google.golang.org/cloud/storage"
  "golang.org/x/net/context"
  "github.com/gorilla/mux"
  "github.com/spf13/viper"
  "encoding/json"
  "github.com/asaskevich/govalidator"
  "mime"
  "os"
  "strconv"
  "math/rand"
)

func main() {

  viper.SetEnvPrefix("IC")
  viper.SetDefault("PORT", "8080")
  viper.AutomaticEnv()
  
  // Check for the required environment settings
  var settings = [...]string {"API_KEY", "GCE_IMAGE_BUCKET", 
                         "GCE_AUTH", "GCE_PROJECT"}
  for _, key := range settings {
    if !viper.IsSet(key) {
        log.Fatal("Need to set environment variable: IC_" + key)
    }
  }
  
  router := mux.NewRouter()
  router.HandleFunc("/", Index)
  log.Fatal(http.ListenAndServe(":" + viper.Get("PORT").(string), router))
}

// Artifact JSON schema returned by the service
type Artifact struct {
  OrigImageURL string  `json:"orig_image_url"`
  OrigHtmlURL string  `json:"orig_html_url"`
  DestImageURL string  `json:"dest_image_url"`
  DestHtmlURL string  `json:"dest_html_url"`
  CompareMetric int  `json:"compare_metric"`
}

// Index is the server entrypoint
func Index(w http.ResponseWriter, r *http.Request) {
  if r.Header.Get("X-API-KEY") != viper.Get("API_KEY") {
    return
  }

  // Origin URL
  orig_url := r.URL.Query().Get("orig_url")
  if !govalidator.IsURL(orig_url) {
    return
  }

  // Destination URL
  dest_url := r.URL.Query().Get("dest_url")
  if !govalidator.IsURL(dest_url) {
    return
  }

  orig_image, orig_html, orig_code := ScreenShot(orig_url)
  //orig_image2, _, _ := ScreenShot(orig_url) // 2nd shot for comparison
  dest_image, dest_html, dest_code := ScreenShot(dest_url)

  if orig_code != 200 || dest_code != 200 {
    return
  }

  var art Artifact
  art.OrigImageURL = StoreArtifact(orig_image, "png")
  art.DestImageURL = StoreArtifact(dest_image, "png")
  art.OrigHtmlURL = StoreArtifact(orig_html, "html")
  art.DestHtmlURL = StoreArtifact(dest_html, "html")

  os.Remove(orig_image)
  os.Remove(dest_image)
  os.Remove(orig_html)
  os.Remove(dest_html)

  data, _ := json.Marshal(art)
  w.Write(data)
}

func makeUniquePath(path string, prefix string, suffix string) (string) {
  ts := strconv.Itoa(int(time.Now().Unix()))
  juice := strconv.Itoa(rand.Int())
  return path + "/" + prefix + "-" + ts + "-" + juice + suffix
}

// Given a url, produce image path, html path, and status code
func ScreenShot(url string) (string, string, int) {
 
  imagePath := makeUniquePath("/tmp", "image", ".png")
  htmlPath := makeUniquePath("/tmp", "html", ".html")
  args := []string{"snap.js", url, "1024", imagePath, htmlPath}
  out, err := exec.Command("/bin/phantomjs", args...).Output()
  if err != nil {
    log.Fatal(err)
  }

  code, _ := strconv.Atoi(string(out))
  return imagePath, htmlPath, code
}

// StoreArtifact saves data to Google Cloud Storage
func StoreArtifact(path string, ext string) string {

  var contentType string

  contentType = mime.TypeByExtension("." + ext)

  data, _ := ioutil.ReadFile(path)

  fileName := fmt.Sprintf("%x-%d.%s", 
                md5.Sum(data), 
                int32(time.Now().Unix()), 
                ext)

  var client *storage.Client
  client = Auth() // for now re-auth every time to be sure
  ctx := context.Background()

  wc := client.Bucket(viper.Get("GCE_IMAGE_BUCKET").(string)).Object(fileName).NewWriter(ctx)

  wc.ContentType = contentType 
  wc.ACL = []storage.ACLRule{{storage.AllUsers, storage.RoleReader}}

  if _, err := wc.Write(data); err != nil {
    log.Fatal(err)
  }
  if err := wc.Close(); err != nil {
    log.Fatal(err)
  }
  client.Close()

  return "https://" + viper.Get("GCE_IMAGE_BUCKET").(string) +
         ".storage.googleapis.com/" + fileName
}

// Auth authenticates Google Cloud Storage.
func Auth() *storage.Client {
  credentials, err := base64.StdEncoding.DecodeString(viper.Get("GCE_AUTH").(string))
  if err != nil {
    log.Fatal(err)
  }
  conf, err := google.JWTConfigFromJSON(
    credentials,
    storage.ScopeFullControl,
  )
  if err != nil {
    log.Fatal(err)
  }
  ctx := context.Background()
  client, err := storage.NewClient(ctx, cloud.WithTokenSource(conf.TokenSource(ctx)))
  if err != nil {
    log.Fatal(err)
  }
  return client
}
