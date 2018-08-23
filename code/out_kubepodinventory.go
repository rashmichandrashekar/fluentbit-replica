package main

import "github.com/fluent/fluent-bit-go/output"
import (
	"fmt"
	"unsafe"
	"C"
	"flag"
	"log"
	"encoding/json"
	"crypto/tls"
	"net/http"
	"bytes"
	"github.com/mitchellh/mapstructure"
)

var (
	// ImageIDMap caches the container id to image mapping
	//ImageIDMap map[string]string

	// NameIDMap caches the container it to Name mapping
	//NameIDMap map[string]string
	
	// KeyFile is the path to the private key file used for auth
	keyFile = flag.String("key", "/etc/opt/microsoft/omsagent/2e8dbed6-141f-4854-a05e-313431fb5887/certs/oms.key", "Private Key File")

	// CertFile is the path to the cert used for auth
	certFile = flag.String("cert", "/etc/opt/microsoft/omsagent/2e8dbed6-141f-4854-a05e-313431fb5887/certs/oms.crt", "OMS Agent Certificate")
)

// DataItem represents the object corresponding to the json that is sent by fluentbit tail plugin
type DataItem struct {
	CollectionTime                string       `json:"CollectionTime"`
	Name                          string       `json:"Name"`
	PodUid                        string       `json:"PodUid"`
	PodLabel                      string       `json:"PodLabel"`
	Namespace                     string       `json:"Namespace"`
	PodCreationTimeStamp          string       `json:"PodCreationTimeStamp"`
	PodStartTime                  string       `json:"PodStartTime"`
	PodStatus                     string       `json:"PodStatus"`
	PodIp                         string       `json:"PodIp"`
	Computer                      string       `json:"Computer"`
	ClusterId                     string       `json:"ClusterId"`
	ClusterName                   string       `json:"ClusterName"`
	ServiceName                   string       `json:"ServiceName"`
	ControllerKind                string       `json:"ControllerKind"`
	ControllerName                string       `json:"ControllerName"`
	PodRestartCount               string       `json:"PodRestartCount"`
	ContainerID                   string       `json:"ContainerID"`
	ContainerName                 string       `json:"ContainerName"`
	ContainerRestartCount         string       `json:"ContainerRestartCount"`
	ContainerStatus               string       `json:"ContainerStatus"`
	ContainerCreationTimeStamp    string       `json:"ContainerCreationTimeStamp"`
}

// KubePodInventoryBlob represents the object corresponding to the payload that is sent to the ODS end point
type KubePodInventoryBlob struct {
	DataType  string     `json:"DataType"`
	IPName    string     `json:"IPName"`
	DataItems []DataItem `json:"DataItems"`
}

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "kubepodinventory", "Stdout GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
func FLBPluginInit(ctx unsafe.Pointer) int {
	// Example to retrieve an optional configuration parameter
	param := output.FLBPluginConfigKey(ctx, "param")
	fmt.Printf("[flb-go] plugin parameter = '%s'\n", param)
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	fmt.Println("Starting the application...")
	var dataItems []DataItem
	for i := 0; i < 10; i++ {
		record := make(map[interface{}]interface{})

		record["CollectionTime"] = "2018-08-22T16:26:45Z"
		record["Name"] = "NEW-KPI-Testing-addon-http-application-routing-default-http-backend-74d4558r79s"
		record["PodUid"] = "aacee011-a4c4-11e8-a30a-0a58ac1f0c13"
		record["PodLabel"] = "[{\"app\"=>\"addon-http-application-routing-default-http-backend\", \"pod-template-hash\"=>\"308011170\"}]"
		record["Namespace"] = "kube-system"
		record["PodCreationTimeStamp"] = "2018-08-22T16:02:02Z"
		record["PodStartTime"] = "2018-08-22T16:02:12Z"
		record["PodStatus"] = "Running"
		record["PodIp"] = "10.244.2.4"
		record["Computer"] = "NEW-KPI-Testing-aks-agentpool-38986853-2"
		record["ClusterId"] = "/subscriptions/692aea0b-2d89-4e7e-ae30-fffe40782ee2/resourceGroups/rashmi-agent-latest/providers/Microsoft.ContainerService/managedClusters/rashmi-agent-latest"
		record["ClusterName"] = "rashmi-agent-latest"
		record["ServiceName"] = "NEW-KPI-Testing-addon-http-application-routing-default-http-backend"
		record["ControllerKind"] = "ReplicaSet"
		record["ControllerName"] = "NEW-KPI-Testing-addon-http-application-routing-default-http-backend-74d4555c4"
		record["PodRestartCount"] = 0
		record["ContainerID"] = "0ce463d2d6f5f44dbfce92ffe50eea3a9d0e0df3610b53515063727f373e6626"
		record["ContainerName"] = "aacee011-a4c4-11e8-a30a-0a58ac1f0c13/addon-http-application-routing-default-http-backend"
		record["ContainerRestartCount"] = 0
		record["ContainerStatus"] = "running"
		record["ContainerCreationTimeStamp"] = "2018-08-22T16:05:41Z"

		var dataItem DataItem

		mapstructure.Decode(record, &dataItem)
		dataItems = append(dataItems, dataItem)
	}

	podEntry := KubePodInventoryBlob{
		DataType:  "KUBE_POD_INVENTORY_BLOB",
		IPName:    "ContainerInsights",
		DataItems: dataItems}

	marshalled, err := json.Marshal(podEntry)

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	url := "https://2e8dbed6-141f-4854-a05e-313431fb5887.ods.opinsights.azure.com/OperationalData.svc/PostJsonDataItems"
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshalled))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	statusCode := resp.Status

	fmt.Println(statusCode)
	fmt.Println("Terminating")

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
