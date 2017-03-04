package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/caarlos0/env"
	"github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	client *http.Client
	pool   *x509.CertPool
)

type envConfig struct {
	ListenPort string `env:"LISTEN_PORT" envDefault:"8080"`
}

//Config stores global env variables
var Config = envConfig{}

var Logger = logrus.New()

func Info(args ...interface{}) {
	Logger.Info(args...)
}

var environment = os.Getenv("ENVIRONMENT")
var region = os.Getenv("REGION")

var bucket = "kubernetes-bitesize-" + environment

var bearerToken, _ = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")

// Global state of test services with a lock
var Lock = struct {
	sync.RWMutex
	State map[string]string
}{State: make(map[string]string)}

const url = "https://kubernetes:443/apis/extensions/v1beta1/namespaces/default/thirdpartyresources/"

func check(e error) {
	if e != nil {
		Logger.Panic(e)
	}
}

func init() {
	Logger.Level = logrus.InfoLevel
	Logger.Formatter = &logrus.JSONFormatter{}
	pool = x509.NewCertPool()
	pool.AppendCertsFromPEM(pemCerts)

	client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, RootCAs: pool}}}
	go func() {
		for _ = range time.Tick(time.Duration(1) * time.Minute) {
			r := RandStringBytes(10)

			tprName := "test-" + r + ".prsn.io"
			createThirdPartyResource(tprName)
		}
	}()
}

func createThirdPartyResource(tpr string) {

	defer deleteThirdPartyResource(tpr)

	var jsonStr = []byte(`{"apiVersion": "extensions/v1beta1","kind": "ThirdPartyResource","description": "test ThirdPartyResource validating stackstorm AWS S3 integration","metadata": {"name": "` + tpr + `","labels": {"type": "testtprs3", "bucket": "` + bucket + `", "region": "` + region + `"}},"versions": [{"name": "v1"}]}`)

	Logger.Info("Create New thirdpartyresource: ", string(jsonStr))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+string(bearerToken))
	resp, err := client.Do(req)
	if err != nil {
		Logger.Error(err)
	}
	Logger.Info(tpr, " ", resp.Status)
	checkS3Object(tpr)

}

func checkS3Object(tpr string) {
	time.Sleep(time.Second * 10)

	testServiceState := "Service Unavailable"

	svc := s3.New(session.New(), &aws.Config{Region: aws.String(region), HTTPClient: client, DisableSSL: aws.Bool(true)})

	result, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("test-tpr-s3"),
	})
	if err != nil {
		Logger.Error(err)
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(result.Body)
		responseStr := buf.String()
		if strings.TrimSpace(string(responseStr)) == tpr {
			testServiceState = "OK"
		}
	}
	Lock.Lock()
	defer Lock.Unlock()
	Lock.State["status"] = testServiceState
	Logger.Info("test service status update: ", testServiceState, " for ", tpr)

}

func deleteThirdPartyResource(tpr string) {

	request, err := http.NewRequest("DELETE", url+tpr, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+string(bearerToken))
	respDelete, err := client.Do(request)
	if err != nil {
		Logger.Error(err)
	}
	Logger.Info(tpr, " thirdparty resource deleted successfully, ", respDelete.Status)
}

func main() {

	configErr := env.Parse(&Config)
	if configErr != nil {
		Logger.Error("%+v\n", configErr)
	}

	Logger.Info("This is the stack: ", environment)
	//	Logger.Info("kubernetes thirdpartyresource endpoint: ", url)

	// Add handlers and start the server
	Address := ":" + Config.ListenPort

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(Logrus())
	router.GET("/", ServiceStatus)
	router.Static("/assets", "./assets")

	s := &http.Server{
		Addr:           Address,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	Logger.Info("Application listening on port ", Config.ListenPort)
	s.ListenAndServe()
}
