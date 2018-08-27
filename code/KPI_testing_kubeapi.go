package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/client-go/kubernetes"
	 "k8s.io/client-go/tools/clientcmd"
	 v1 "k8s.io/api/core/v1"
	 "k8s.io/apimachinery/pkg/types"
	 "strings"
	 "os"
	 //"reflect"

	"github.com/mitchellh/mapstructure"
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

//type MapInterface map[interface{}]interface{}

var ClusterId string
var ClusterName string
var clientset *kubernetes.Clientset

func getServiceNameFromLabels(namespace string, labels map[string]string, serviceList *v1.ServiceList) string {
	serviceName := ""
	if labels !=nil {
		if serviceList != nil {
			for _, service := range serviceList.Items {
				found := 0
				if service.Spec.Selector != nil && service.ObjectMeta.Namespace == namespace {
					selectorLabels := service.Spec.Selector
					if selectorLabels != nil {
						for key, value := range selectorLabels {
							if labels[key] == value {
								//break
								found = found + 1
							}
							//found = found + 1
						}
					}
					if found == len(selectorLabels) {
						fmt.Println(service.ObjectMeta.Name)
						return service.ObjectMeta.Name
					}
				}
			}
		}
	}
	return serviceName
}

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

func parseAndEmitRecords(pods *v1.PodList, services *v1.ServiceList) {
	//clusterName := getClusterName()
	//fmt.Println(clusterName)
	//currentTime = Time.now
	var dataItems []DataItem
	for _, pod := range pods.Items {
		currentTime := time.Now()
		//fmt.Println(currentTime)
		batchTime := currentTime.UTC().Format(time.RFC3339)
		//emitTime := currentTime.to_f
		//batchTime := currentTime.UTC.iso8601
		//var dataItems []DataItem
		record := make(map[interface{}]interface{})
		//recorddup := make(map[interface{}]interface{})
		//var records []MapInterface
		var dataItem DataItem

		//This is the time that is mapped to become TimeGenerated
		record["CollectionTime"] = batchTime 
		podmetadata := pod.ObjectMeta
		record["Name"] = podmetadata.Name + "-rashmicomplete"
		podNameSpace := podmetadata.Namespace
		record["Namespace"] = podNameSpace
		podLabels, err := json.Marshal(podmetadata.Labels)
		if (err == nil) {
			podLabelString := "[" + string(podLabels) + "]"
			record["PodLabel"] = podLabelString
		}
		//fmt.Println(podmetadata.Labels)
		var podUid types.UID
		//podannotations := podmetadata.Annotations
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
		record["PodUid"] = podUid
		//fmt.Println(podannotations)
		// TODO: get pod uid
		//podUid := podmetadata.UID
		record["PodCreationTimeStamp"] = podmetadata.CreationTimestamp.Format(time.RFC3339)
		podstarttime := pod.Status.StartTime
		//for unscheduled (non-started) pods startTime does NOT exist
		if podstarttime != nil {
			record["PodStartTime"] = podstarttime.Format(time.RFC3339)
		} else {
			record["PodStartTime"] = ""
		}
		//podStatus
		//the below is for accounting 'NodeLost' scenario, where-in the pod(s) in the lost node is still being reported as running
		podReadyCondition := true
		podStatusReason := pod.Status.Reason
		if podStatusReason != "" && podStatusReason == "NodeLost" {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "False" {
					podReadyCondition = false
					break
				}
			}
		}

		if podReadyCondition == false {
			record["PodStatus"] = "Unknown"
		} else {
			record["PodStatus"] = pod.Status.Phase
		}

		//for unscheduled (non-started) pods podIP does NOT exist
		if pod.Status.PodIP != "" {
            record["PodIp"] = pod.Status.PodIP
		} else {
			record["PodIp"] = ""
        }

		//for unscheduled (non-started) pods nodeName does NOT exist
        if pod.Spec.NodeName != "" {
            record["Computer"] = pod.Spec.NodeName
		} else {
            record["Computer"] = ""
        } 
		record["ClusterId"] = getClusterId()
		record["ClusterName"] = getClusterName()
		//record["ServiceName"] = "my_service_name"
		//fmt.Println (reflect.TypeOf(podmetadata.Labels))

		record["ServiceName"] = getServiceNameFromLabels(podNameSpace, podmetadata.Labels, services)
		if podmetadata.OwnerReferences != nil {
			record["ControllerKind"] = podmetadata.OwnerReferences[0].Kind
            record["ControllerName"] = podmetadata.OwnerReferences[0].Name
		}
		podRestartCount := 0
        record["PodRestartCount"] = podRestartCount
		if pod.Status.ContainerStatuses != nil && len(pod.Status.ContainerStatuses) > 0 {
			for _, container := range pod.Status.ContainerStatuses {
				containerRestartCount := 0
				//container Id is of the form 		
                //docker://dfd9da983f1fd27432fb2c1fe3049c0a1d25b1c697b2dc1a530c986e58b16527
				if container.ContainerID != "" {
					record["ContainerID"] = strings.Split(container.ContainerID, "//")[1]
				} else {
					record["ContainerID"] = "00000000-0000-0000-0000-000000000000"  
				}
				//keeping this as <PodUid/container_name> which is same as InstanceName in perf table
				//podUidString := (string)podUid
				record["ContainerName"] = string(podUid) + "/" + container.Name
				//Pod restart count is a sumtotal of restart counts of individual containers		
                //within the pod. The restart count of a container is maintained by kubernetes		
                //itself in the form of a container label.
				containerRestartCount = int(container.RestartCount)
                record["ContainerRestartCount"] = containerRestartCount
                containerStatus := container.State
				//state is of the following form , so just picking up the first key name
                //"state": {
                //	"waiting": {
                //		"reason": "CrashLoopBackOff",
                //		"message": "Back-off 5m0s restarting failed container=metrics-server pod=metrics-server-2011498749-3g453_kube-system(5953be5f-fcae-11e7-a356-000d3ae0e432)"
                //	}
                //},
                //the below is for accounting 'NodeLost' scenario, where-in the containers in the lost node/pod(s) is still being reported as running
				if podReadyCondition == false {
					record["ContainerStatus"] = "Unknown"
				} else {
					if containerStatus.Running != nil {
						record["ContainerStatus"] = "Running"
						record["ContainerCreationTimeStamp"] = containerStatus.Running.StartedAt.Format(time.RFC3339)
					} else if containerStatus.Terminated != nil {
						record["ContainerStatus"] = "Terminated"
					} else if containerStatus.Waiting != nil {
						record["ContainerStatus"] = "Waiting"
					}
					//fmt.Println(containerStatus.Running.StartedAt)
					//fmt.Println(containerStatus.Terminated.FinishedAt)
					//fmt.Println(containerStatus.Waiting.Message)
				}
				//TODO : Remove ContainerCreationTimeStamp from here since we are sending it as a metric
				//Picking up both container and node start time from cAdvisor to be consistent
				/*if container.State.Running.StartedAt != "" {
					record["ContainerCreationTimeStamp"] = container.State.Running.StartedAt
				}*/
				podRestartCount += containerRestartCount
				//for index,element := range record{        
				//	recorddup[index] = element
				//}
				//copy(recorddup, record)
				//records = append(records,recorddup)

				mapstructure.Decode(record, &dataItem)
				dataItems = append(dataItems, dataItem)
			} 
			} else {
				//records = append(record)
				mapstructure.Decode(record, &dataItem)
				dataItems = append(dataItems, dataItem)
			}

			//var dataItem DataItem

			//mapstructure.Decode(record, &dataItem)
			//dataItems = append(dataItems, dataItem)
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

		//fmt.Println(record)
		//fmt.Println(pod)
	}

func main() {
	fmt.Println("Starting the application...")
	//fmt.Println(time.Now())
	config, err := clientcmd.BuildConfigFromFlags("", "C:\\Users\\rashmy\\.kube\\config")
	if err != nil {
		return
	}
	clientset, err = kubernetes.NewForConfig(config)
	//fmt.Println (reflect.TypeOf(clientset))
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
	//fmt.Println(time.Now())
	fmt.Println("Terminating")
}