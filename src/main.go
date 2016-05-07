package main

import (
  "crypto/md5"
  "encoding/base64"
  "fmt"
  "log"
  "net/http"
  "time"
  "golang.org/x/oauth2/google"
  "google.golang.org/cloud"
  "google.golang.org/cloud/storage"
  "golang.org/x/net/context"
  "github.com/gorilla/mux"
  "github.com/spf13/viper"
  "sourcegraph.com/sourcegraph/go-selenium"
  "encoding/json"
  "github.com/asaskevich/govalidator"
  "mime"
)

func main() {

  viper.SetEnvPrefix("IC")
  viper.SetDefault("PORT", "8080")
  viper.SetDefault("GRID_PORT", "4444")
  viper.AutomaticEnv()
  
  // Check for the required environment settings
  var settings = [...]string {"API_KEY", "GCE_IMAGE_BUCKET", 
                         "GCE_AUTH", "GCE_PROJECT", "GRID_IP"} 
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
  URL string `json:"url"`
  ImageURL string  `json:"image"`
  HtmlURL string  `json:"html"`
  Code int  `json:"code"`
}

// Index is the server entrypoint
func Index(w http.ResponseWriter, r *http.Request) {
  if r.Header.Get("X-API-KEY") != viper.Get("API_KEY") {
    return
  }

  // artifact target url
  url := r.URL.Query().Get("url")
  if !govalidator.IsURL(url) {
    return
  }

  art := GetArtifact(url)
  data, _ := json.Marshal(art)
  w.Write(data)
}

func GetArtifact(url string) Artifact {
  imageData, htmlData, code := ScreenShot(url)

  var art Artifact
  art.URL = url
  art.Code = code
  art.ImageURL = StoreArtifact(imageData, "png")
  art.HtmlURL = StoreArtifact([]byte(htmlData), "html")
  return art
}

// ScreenShot produces a png screenshot, html, and http code for a url
func ScreenShot(url string) ([]byte, string, int) {
  var wd selenium.WebDriver
  var err error
  var data []byte = nil
  var html string = ""
  var code int = 0

  gridURL := fmt.Sprintf("http://%s:%s/wd/hub", viper.Get("GRID_IP"), viper.Get("GRID_PORT"))

  caps := selenium.Capabilities(map[string]interface{}{"browserName": "firefox"})

  if wd, err = selenium.NewRemote(caps, gridURL); err != nil {
    panic(err)
  }

  if err = wd.Get(url); err != nil {
    data = nil
  }

/* Bugged in Chrome, not needed with FireFox
  if window, err = wd.CurrentWindowHandle(); err != nil {
    to := selenium.Size{Width: 1024, Height: 5000}
    wd.ResizeWindow(window, to)
  }
*/

  time.Sleep(15 * time.Second)

  if data, err = wd.Screenshot(); err != nil {
    data = nil
  }

  if html, err = wd.PageSource(); err != nil {
    html = ""
  }

  var resp *http.Response
  timeout := time.Duration(60 * time.Second)
  client := http.Client {
    Timeout: timeout,
  }

  if resp, err = client.Get(url); err != nil {
    fmt.Print("Timeout for http get")
  } else {
    code = resp.StatusCode
    defer resp.Body.Close()
  }

  return data, html, code
}

// StoreArtifact saves data to Google Cloud Storage
func StoreArtifact(data []byte, ext string) string {

  var contentType string

  contentType = mime.TypeByExtension("." + ext)

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

  if _, err := wc.Write([]byte(data)); err != nil {
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
