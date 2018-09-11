package main

import (
	//"bytes"
	//"crypto/tls"
	//"encoding/json"
	"flag"
	"fmt"
	//"log"
	//"net/http"
	"time"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/client-go/kubernetes"
	 "k8s.io/client-go/tools/clientcmd"
	 v1 "k8s.io/api/core/v1"
	 //"reflect"

	//"github.com/mitchellh/mapstructure"
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
	PodRestartCount               int          `json:"PodRestartCount"`
	ContainerID                   string       `json:"ContainerID"`
	ContainerName                 string       `json:"ContainerName"`
	ContainerRestartCount         int          `json:"ContainerRestartCount"`
	ContainerStatus               string       `json:"ContainerStatus"`
	ContainerCreationTimeStamp    string       `json:"ContainerCreationTimeStamp"`
}

// KubePodInventoryBlob represents the object corresponding to the payload that is sent to the ODS end point
type KubePodInventoryBlob struct {
	DataType  string     `json:"DataType"`
	IPName    string     `json:"IPName"`
	DataItems []DataItem `json:"DataItems"`
}

var (
	certFile = flag.String("cert", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\oms.crt", "OMS Agent Certificate")
	keyFile  = flag.String("key", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\oms.key", "Certificate Private Key")
)

func parseAndEmitRecords(pods *v1.PodList, services *v1.ServiceList) {
	//currentTime = Time.now
	for _, pod := range pods.Items {
		//currentTime := time.Now()
		//emitTime := currentTime.to_f
		//batchTime := currentTime.utc.iso8601
		//var dataItems []DataItem
		record := make(map[interface{}]interface{})
		//This is the time that is mapped to become TimeGenerated
		//record["CollectionTime"] = batchTime 
		podmetadata := pod.ObjectMeta
		record["Name"] = podmetadata.Name
		record["Namespace"] = podmetadata.Namespace
		//fmt.Println(reflect.TypeOf(pod))
		fmt.Println(record)
		//fmt.Println(pod)
	}

}

func main() {
	fmt.Println("Starting the application...")
	fmt.Println(time.Now())
	config, err := clientcmd.BuildConfigFromFlags("", "C:\\Users\\rashmy\\.kube\\config")
	if err != nil {
		return
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	/*for _, pod := range pods.Items {
		fmt.Println(pod)

		for _, status := range pod.Status.ContainerStatuses {
			fmt.Printf("Pod Name %s --> Container ID %s \n", pod.Name, status.ContainerID)
		}
	}*/

	services, err := clientset.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	parseAndEmitRecords(pods, services)

	/*var dataItems []DataItem
	for i := 0; i < 10; i++ {
		record := make(map[interface{}]interface{})

		record["CollectionTime"] = "2018-08-22T10:26:45Z"
		record["Name"] = "KPI-Testing-addon-http-application-routing-default-http-backend-74d4558r79s"
		record["PodUid"] = "aacee011-a4c4-11e8-a30a-0a58ac1f0c13"
		record["PodLabel"] = "[{\"app\"=>\"addon-http-application-routing-default-http-backend\", \"pod-template-hash\"=>\"308011170\"}]"
		record["Namespace"] = "kube-system"
		record["PodCreationTimeStamp"] = "2018-08-22T10:02:02Z"
		record["PodStartTime"] = "2018-08-22T10:02:12Z"
		record["PodStatus"] = "Running"
		record["PodIp"] = "10.244.2.4"
		record["Computer"] = "KPI-Testing-aks-agentpool-38986853-2"
		record["ClusterId"] = "/subscriptions/692aea0b-2d89-4e7e-ae30-fffe40782ee2/resourceGroups/rashmi-agent-latest/providers/Microsoft.ContainerService/managedClusters/rashmi-agent-latest"
		record["ClusterName"] = "rashmi-agent-latest"
		record["ServiceName"] = "KPI-Testing-addon-http-application-routing-default-http-backend"
		record["ControllerKind"] = "ReplicaSet"
		record["ControllerName"] = "KPI-Testing-addon-http-application-routing-default-http-backend-74d4555c4"
		record["PodRestartCount"] = 0
		record["ContainerID"] = "0ce463d2d6f5f44dbfce92ffe50eea3a9d0e0df3610b53515063727f373e6626"
		record["ContainerName"] = "aacee011-a4c4-11e8-a30a-0a58ac1f0c13/addon-http-application-routing-default-http-backend"
		record["ContainerRestartCount"] = 0
		record["ContainerStatus"] = "running"
		record["ContainerCreationTimeStamp"] = "2018-08-22T10:05:41Z"

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

	fmt.Println(statusCode)*/
	fmt.Println(time.Now())
	fmt.Println("Terminating")
}