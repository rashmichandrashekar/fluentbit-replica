package main

//import "github.com/fluent/fluent-bit-go/output"
import (
	"fmt"
	"C"
	"flag"
	"log"
	"encoding/json"
	"crypto/tls"
	"net/http"
	"bytes"
	"time"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/client-go/kubernetes"
	 "k8s.io/client-go/tools/clientcmd"
	 v1 "k8s.io/api/core/v1"
	 "k8s.io/apimachinery/pkg/types"
	 "strings"
	 "os"
	"github.com/mitchellh/mapstructure"
	"math"
	//"os/exec"
	//"reflect"
	//"github.com/nu7hatch/gouuid"
)

/*var (
	certFile = flag.String("cert", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\KubePerf\\rashmi-kube-perf-workspace\\oms.crt", "OMS Agent Certificate")
	keyFile  = flag.String("key", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\KubePerf\\rashmi-kube-perf-workspace\\oms.key", "Certificate Private Key")*/
var (
	certFile = flag.String("cert", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\oms.crt", "OMS Agent Certificate")
	keyFile  = flag.String("key", "C:\\Users\\rashmy\\Documents\\ReplicaSetFluentBit\\oms.key", "Certificate Private Key")
)


// DataItem represents the object corresponding to the json that is sent by fluentbit tail plugin
type DataItem struct {
	Timestamp        string                   `json:"Timestamp"`
	Host             string                   `json:"Host"`
	ObjectName       string                   `json:"ObjectName"`
	InstanceName     string                   `json:"InstanceName"`
	Collections      []MetricCollection       `json:"Collections"`
}

type MetricCollection struct {
	CounterName       string       `json:"CounterName"`
	Value             float64      `json:"Value"`
}


// KubePerfBlob represents the object corresponding to the payload that is sent to the ODS end point
type KubePerfBlob struct {
	DataType  string     `json:"DataType"`
	IPName    string     `json:"IPName"`
	DataItems []DataItem `json:"DataItems"`
}

var ClusterId string
var ClusterName string
var clientset *kubernetes.Clientset
var NodeMetrics map[string]float64


func getClusterName() string {
	if (ClusterName != "") {
		return ClusterName
	}
	ClusterName = "None"
	//try getting resource ID for aks 
	cluster := os.Getenv("AKS_RESOURCE_ID")
	if cluster != "" {
		ClusterNameSplit := strings.Split(cluster, "/")
		ClusterName = ClusterNameSplit[len(ClusterNameSplit) - 1]
	} else {
		cluster = os.Getenv("ACS_RESOURCE_NAME")
		if cluster != "" {
			ClusterName = cluster
		} else {
			pods, err := clientset.CoreV1().Pods("kube-system").List(metav1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}
			for _, pod := range pods.Items {
				podMetadataName := pod.ObjectMeta.Name
				if strings.Contains(podMetadataName, "kube-controller-manager") {
					for _, podSpecContainerCommand := range pod.Spec.Containers[0].Command {
						if strings.Contains(podSpecContainerCommand, "--cluster-name") {
							commandSplit := strings.Split (podSpecContainerCommand, "=")
							ClusterName = commandSplit[1]
						}
					}
				}
			}
		}
	}
	return ClusterName
} 

func getClusterId() string{
	if ClusterId != "" {
		return ClusterId
	}
	//By default initialize ClusterId to ClusterName. 
    //<TODO> In ACS/On-prem, we need to figure out how we can generate ClusterId
    ClusterId = getClusterName()
	cluster := os.Getenv("AKS_RESOURCE_ID")
	if cluster != "" {
		ClusterId = cluster
	}
    return ClusterId
}

func getContainerResourceRequestsAndLimits(pods *v1.PodList, metricCategory string, metricNameToCollect string, metricNametoReturn string) []DataItem{
	var metricItems []DataItem
	clusterId := getClusterId()
	for _, pod := range pods.Items {
		var metricItem DataItem
		var metricCounterItems []MetricCollection
		
		podmetadata := pod.ObjectMeta
		podNameSpace := podmetadata.Namespace
		var podUid types.UID
		if podNameSpace == "kube-system" && podmetadata.OwnerReferences == nil {
            // The above case seems to be the only case where you have horizontal scaling of pods
            // but no controller, in which case cAdvisor picks up kubernetes.io/config.hash
            // instead of the actual poduid. Since this uid is not being surface into the UX
            // its ok to use this.
            // Use kubernetes.io/config.hash to be able to correlate with cadvisor data
				podUid = types.UID(podmetadata.Annotations["kubernetes.io/config.hash"])
				//['kubernetes.io/config.hash']
		} else {
			podUid = podmetadata.UID
		}

		if pod.Spec.Containers != nil && pod.Spec.NodeName != "" {
			nodeName := pod.Spec.NodeName
			record := make(map[interface{}]interface{})
			for _, container := range pod.Spec.Containers {
				containerName := container.Name
				currentTime := time.Now()
				metricTime := currentTime.UTC().Format(time.RFC3339)
				var metricValue float64
				switch metricNameToCollect {
					case "cpu": if metricCategory == "limits" {
									if container.Resources.Limits.Cpu != nil {
										//Results returned in cores. Converting them to nanocores
										containerMetricValue := container.Resources.Limits.Cpu().Value()
										metricValueMultiplier := math.Pow(1000, 3)
										metricValue = float64(containerMetricValue) * metricValueMultiplier
									} else {
										nodeMetricsHashKey := clusterId + "/" + nodeName + "_" + "allocatable" +  "_" + metricNameToCollect
										if NodeMetrics != nil && NodeMetrics[nodeMetricsHashKey] != 0 {
											metricValue = NodeMetrics[nodeMetricsHashKey]
										}
									}
								} else if metricCategory == "requests" {
									containerMetricValue := container.Resources.Requests.Cpu().Value()
									metricValueMultiplier := math.Pow(1000, 3)
									metricValue = float64(containerMetricValue) * metricValueMultiplier
								}
					case "memory": if metricCategory == "limits" {
										metricValue = float64(container.Resources.Limits.Memory().Value())
									} else if metricCategory == "requests" {
										metricValue = float64(container.Resources.Requests.Memory().Value())
									}
				}
				record["Timestamp"] = metricTime
				record["Host"] = nodeName
				record["ObjectName"] = "K8SContainer"
				record["InstanceName"] = clusterId + "/" + string(podUid)+ "/" + containerName
				metricCounter := MetricCollection {
					CounterName: metricNametoReturn,
					Value: metricValue,
				}
				metricCounterItems = append(metricCounterItems, metricCounter)
				record["Collections"] = metricCounterItems

				mapstructure.Decode(record, &metricItem)
				metricItems = append(metricItems, metricItem)
			}
		}
	}
	return metricItems
}

func parseNodeLimits(nodes *v1.NodeList, metricCategory string, metricNameToCollect string, metricNametoReturn string) []DataItem {
	var metricItems []DataItem
	clusterId := getClusterId()
	currentTime := time.Now()
	metricTime := currentTime.UTC().Format(time.RFC3339)
	for _, node := range nodes.Items {
		var metricItem DataItem
		var metricCounterItems []MetricCollection

		nodeMetaData := node.ObjectMeta
		nodeName := nodeMetaData.Name
		var metricValue float64
			switch metricNameToCollect {
				case "cpu": if metricCategory == "allocatable" {
								nodeMetricValue := node.Status.Allocatable.Cpu().Value()
								metricValueMultiplier := math.Pow(1000, 3)
								metricValue = float64(nodeMetricValue) * metricValueMultiplier
							} else if metricCategory == "capacity" {
								nodeMetricValue := node.Status.Capacity.Cpu().Value()
								metricValueMultiplier := math.Pow(1000, 3)
								metricValue = float64(nodeMetricValue) * metricValueMultiplier
							}
				case "memory": if metricCategory == "allocatable" {
									metricValue = float64(node.Status.Allocatable.Memory().Value())
								} else if metricCategory == "capacity" {
									metricValue = float64(node.Status.Capacity.Memory().Value())
								}
			}
			record := make(map[interface{}]interface{})
			record["Timestamp"] = metricTime
			record["Host"] = nodeName
			record["ObjectName"] = "K8SNode"
			record["InstanceName"] = clusterId + "/" + nodeName
			metricCounter := MetricCollection {
				CounterName: metricNametoReturn,
				Value: metricValue,
			}
			metricCounterItems = append(metricCounterItems, metricCounter)
			record["Collections"] = metricCounterItems
			
			mapstructure.Decode(record, &metricItem)
			metricItems = append(metricItems, metricItem)
			//push node level metrics to a inmem hash so that we can use it looking up at container level.
			//Currently if container level cpu & memory limits are not defined we default to node level limits
			NodeMetrics := make(map[string]float64)
			NodeMetrics[clusterId + "/" + nodeName + "_" + metricCategory + "_" + metricNameToCollect] = metricValue
	}
	return metricItems
}

func enumerate() {
	/*config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}*/
	fmt.Println("Getting pods...")
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Done getting pods...")
	var dataItems []DataItem
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "requests", "cpu","cpuRequestNanoCores")...)
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "requests", "memory","memoryRequestBytes")...)
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "limits", "cpu","cpuLimitNanoCores")...)
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "limits", "memory","memoryLimitBytes")...)

	fmt.Println("Getting nodes...")

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Done getting nodes...")
	dataItems = append(dataItems, parseNodeLimits(nodes, "allocatable", "cpu", "cpuAllocatableNanoCores")...)
	dataItems = append(dataItems, parseNodeLimits(nodes, "allocatable", "memory", "memoryAllocatableBytes")...)
	dataItems = append(dataItems, parseNodeLimits(nodes, "capacity", "cpu", "cpuCapacityNanoCores")...)
	dataItems = append(dataItems, parseNodeLimits(nodes, "capacity", "memory", "memoryCapacityBytes")...)

	podEntry := KubePerfBlob{
		DataType:  "LINUX_PERF_BLOB",
		IPName:    "LogManagement",
		DataItems: dataItems,
	}

	marshalled, err := json.Marshal(podEntry)
	//fmt.Println(podEntry)
	
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	//url := "https://6d3a50d0-808e-4d66-86c0-7b99d810ffe1.ods.opinsights.azure.com/OperationalData.svc/PostJsonDataItems"
	url := "https://fed8f683-b8a5-452f-9191-beb989ad4b76.ods.opinsights.azure.com/OperationalData.svc/PostJsonDataItems"
	//url := "https://2e8dbed6-141f-4854-a05e-313431fb5887.ods.opinsights.azure.com/OperationalData.svc/PostJsonDataItems"

	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshalled))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	statusCode := resp.Status
	fmt.Println(statusCode)
}

func main() {
	fmt.Println("Starting the application...")
	config, err := clientcmd.BuildConfigFromFlags("", "C:\\Users\\rashmy\\.kube\\config")
	if err != nil {
		return
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return
	}
	enumerate()
		
	fmt.Println(time.Now())
	fmt.Println("Terminating")
}
