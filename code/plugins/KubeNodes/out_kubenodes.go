package main

import "github.com/fluent/fluent-bit-go/output"
import (
	"fmt"
	"C"
	"flag"
	"log"
	"encoding/json"
	"unsafe"
	"crypto/tls"
	"net/http"
	"bytes"
	"time"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/client-go/kubernetes"
	 "strings"
	 "os"
	"github.com/mitchellh/mapstructure"
	"k8s.io/client-go/rest"
	"reflect"
)

var (
	// KeyFile is the path to the private key file used for auth
	keyFile = flag.String("key", "/etc/opt/microsoft/omsagent/6d3a50d0-808e-4d66-86c0-7b99d810ffe1/certs/oms.key", "Private Key File")

	// CertFile is the path to the cert used for auth
	certFile = flag.String("cert", "/etc/opt/microsoft/omsagent/6d3a50d0-808e-4d66-86c0-7b99d810ffe1/certs/oms.crt", "OMS Agent Certificate")
)

// DataItem represents the object corresponding to the json that is sent by fluentbit tail plugin
type DataItem struct {
	CollectionTime              string                   `json:"CollectionTime"`
	Computer                    string                   `json:"Computer"`
	ClusterName                 string                   `json:"ClusterName"`
	ClusterId                   string                   `json:"ClusterId"`
	CreationTimeStamp           string                   `json:"CreationTimeStamp"`
	Labels                      string                   `json:"Labels"`
	Status                      string                   `json:"Status"`
	LastTransitionTimeReady     string                   `json:"LastTransitionTimeReady"`
	KubeletVersion              string                   `json:"KubeletVersion"`
	KubeProxyVersion            string                   `json:"KubeProxyVersion"`
}

// KubeNodeInventoryBlob represents the object corresponding to the payload that is sent to the ODS end point
type KubeNodeInventoryBlob struct {
	DataType  string     `json:"DataType"`
	IPName    string     `json:"IPName"`
	DataItems []DataItem `json:"DataItems"`
}

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "kubenodes", "Stdout GO!")
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

var ClusterId string
var ClusterName string
var clientset *kubernetes.Clientset

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

func enumerate() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Getting nodes...")

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Done getting nodes...")
	var nodeItems []DataItem
	//clusterId := getClusterId()
	currentTime := time.Now()
	fmt.Println(reflect.TypeOf(currentTime))
	batchTime := currentTime.UTC().Format(time.RFC3339)
	for _, node := range nodes.Items {
		var nodeItem DataItem
		nodeMetaData := node.ObjectMeta
		record := make(map[interface{}]interface{})
		//This is the time that is mapped to become TimeGenerated
		record["CollectionTime"] = batchTime
		record["Computer"] = nodeMetaData.Name
		record["ClusterName"] = getClusterName()
		record["ClusterId"] = getClusterId()
		record["CreationTimeStamp"] = nodeMetaData.CreationTimestamp.Format(time.RFC3339)
		nodeLabelsField, err := json.Marshal(nodeMetaData.Labels)
		if (err == nil) {
			nodeLabelsString := "[" + string(nodeLabelsField) + "]"
			record["Labels"] = nodeLabelsString
		}

		record["Status"] = ""
		//Refer to https://kubernetes.io/docs/concepts/architecture/nodes/#condition for possible node conditions.
        //We check the status of each condition e.g. {"type": "OutOfDisk","status": "False"} . Based on this we 
        //populate the KubeNodeInventory Status field. A possible value for this field could be "Ready OutofDisk"
        //implying that the node is ready for hosting pods, however its out of disk.
		if len(node.Status.Conditions) > 0 {
			allNodeConditions := ""
			for _, condition := range node.Status.Conditions {
				if condition.Status == "True" {
					if (len(allNodeConditions) > 0) {
						allNodeConditions = allNodeConditions + "," + string(condition.Type)
					} else {
						allNodeConditions = string(condition.Type)
					}
				}
				//collect last transition to/from ready (no matter ready is true/false)
				if condition.Type == "Ready" && condition.LastTransitionTime != (metav1.Time{}) {
					record["LastTransitionTimeReady"] = condition.LastTransitionTime.Format(time.RFC3339)
				}
			}
			if len(allNodeConditions) > 0 {
				record["Status"] = allNodeConditions
			}
		}
		
		record["KubeletVersion"] = node.Status.NodeInfo.KubeletVersion
		record["KubeProxyVersion"] = node.Status.NodeInfo.KubeProxyVersion
			
		mapstructure.Decode(record, &nodeItem)
		nodeItems = append(nodeItems, nodeItem)
			
	}
	nodeEntry := KubeNodeInventoryBlob{
		DataType:  "KUBE_NODE_INVENTORY_BLOB",
		IPName:    "ContainerInsights",
		DataItems: nodeItems,
	}

	marshalled, err := json.Marshal(nodeEntry)
	
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	url := "https://6d3a50d0-808e-4d66-86c0-7b99d810ffe1.ods.opinsights.azure.com/OperationalData.svc/PostJsonDataItems"
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshalled))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	statusCode := resp.Status
	fmt.Println(statusCode)
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	fmt.Println("Kubenodes-Starting the application...")
	enumerate()
	fmt.Println("Kubenodes-Terminating")

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}

