package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/caarlos0/env"
	"github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	//	"io/ioutil"
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
var stackId = os.Getenv("STACK_ID")

// Global state of test services with a lock
var Lock = struct {
	sync.RWMutex
	State map[string]string
}{State: make(map[string]string)}

var url = "https://stackstorm-" + stackId + "." + environment + "." + "kube/api/v1/executions/"

const St2ApiKey = "NzlhYTFjNjE5ZGZhMTk1NGQxYzYzNzMwYTJjMTJiN2Y0OTg0MjJjMmJjMTNhNjdjY2QzNGUwZDU1NDQ5MmQ4MQ"

func init() {
	Logger.Level = logrus.InfoLevel
	Logger.Formatter = &logrus.JSONFormatter{}
	pool = x509.NewCertPool()
	pool.AppendCertsFromPEM(pemCerts)

	client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, RootCAs: pool}}}
	go func() {
		for _ = range time.Tick(time.Duration(120) * time.Second) {

			executeStackstormAction()
		}
	}()
}

func executeStackstormAction() {

	testServiceState := "Service Unavailable"

	var jsonStr = []byte(`{"action": "bitesize.check_st2_sensor"}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("St2-Api-Key", St2ApiKey)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		Logger.Error(err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	responseStr := buf.String()
	newResp := []byte(responseStr)

	type St2ResultData struct {
		Result   string
		ExitCode int
		StdErr   string
		StdOut   string
	}

	var St2Response struct {
		Status         string        `json:"status"`
		ExecutionId    string        `json:"id"`
		StartTimestamp string        `json:"start_timestamp"`
		Log            interface{}   `json:"log"`
		Context        interface{}   `json:"context"`
		Runner         interface{}   `json:"runner"`
		WebUrl         string        `json:"web_url"`
		Action         interface{}   `json:"action"`
		Liveaction     interface{}   `json:"liveaction"`
		Result         St2ResultData `json:"result,omitempty"`
		ElapsedSeconds float64       `json:"elapsed_seconds,omitempty"`
		EndTimeStamp   string        `json:"end_timestamp,omitempty"`
	}

	respErr := json.Unmarshal(newResp, &St2Response)
	if respErr != nil {
		fmt.Println("error:", respErr)
	}

	Logger.Info("New st2 action execution id: ", St2Response.ExecutionId)
	time.Sleep(time.Second * 2)
	resultUrl := url + St2Response.ExecutionId

	request, err := http.NewRequest("GET", resultUrl, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("St2-Api-Key", St2ApiKey)
	response, err := client.Do(request)
	defer response.Body.Close()
	if err != nil {
		Logger.Error(err)
	}

	buffer := new(bytes.Buffer)
	buffer.ReadFrom(response.Body)
	resultStr := buffer.String()
	newResult := []byte(resultStr)
	resultErr := json.Unmarshal(newResult, &St2Response)
	if resultErr != nil {
		fmt.Println("error:", resultErr)
	}

	Logger.Info(St2Response.ExecutionId, " result: ", St2Response.Result.Result)

	if strings.Contains(St2Response.Result.Result, "failed") {
		Logger.Error("At least one stackstorm sensor has failed.")
	} else {
		testServiceState = "OK"
	}
	Lock.Lock()
	defer Lock.Unlock()
	Lock.State["status"] = testServiceState
}

func main() {

	configErr := env.Parse(&Config)
	if configErr != nil {
		Logger.Error("%+v\n", configErr)
	}

	// Add handlers and start the server
	Address := ":" + Config.ListenPort

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(Logrus())
	router.GET("/", ServiceStatus)

	s := &http.Server{
		Addr:           Address,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	Logger.Info("Application listening on port ", Config.ListenPort)
	Logger.Info("Stackstorm API endpoint: ", url)
	s.ListenAndServe()
}
