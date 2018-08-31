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
	"time"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/client-go/kubernetes"
	 //"k8s.io/client-go/tools/clientcmd"
	 v1 "k8s.io/api/core/v1"
	 "k8s.io/apimachinery/pkg/types"
	 "strings"
	 "os"
	"github.com/mitchellh/mapstructure"
	"k8s.io/client-go/rest"
	"math"
	"strconv"
)

var (
	// KeyFile is the path to the private key file used for auth
	keyFile = flag.String("key", "/etc/opt/microsoft/omsagent/6d3a50d0-808e-4d66-86c0-7b99d810ffe1/certs/oms.key", "Private Key File")

	// CertFile is the path to the cert used for auth
	certFile = flag.String("cert", "/etc/opt/microsoft/omsagent/6d3a50d0-808e-4d66-86c0-7b99d810ffe1/certs/oms.crt", "OMS Agent Certificate")
)


// DataItem represents the object corresponding to the json that is sent by fluentbit tail plugin
type DataItem struct {
	Timestamp        string       `json:"Timestamp"`
	Host             string       `json:"Host"`
	ObjectName       string       `json:"ObjectName"`
	InstanceName     string       `json:"InstanceName"`
	Collections      string       `json:"Collections"`
}

type MetricCollection struct {
	CounterName       string       `json:"CounterName"`
	Value             float64      `json:"Value"`
}


// KubePodInventoryBlob represents the object corresponding to the payload that is sent to the ODS end point
type KubePerfBlob struct {
	DataType  string     `json:"DataType"`
	IPName    string     `json:"IPName"`
	DataItems []DataItem `json:"DataItems"`
}

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "kubeperf", "Stdout GO!")
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
var NodeMetrics map[interface{}]interface{}

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

func getMetricNumericValue(metricName string, metricVal string) float64{
	//metricValue := metricVal
	var metricValue float64
	switch metricName {
		//when "memory" #convert to bytes for memory
		//https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/
		case "memory" :	if strings.HasSuffix(metricVal, "Ki") {
							strings.TrimSuffix(metricVal, "Ki")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
							metricValue = metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 1)
						} else if strings.HasSuffix(metricVal, "Mi") {
							strings.TrimSuffix(metricVal, "Mi")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
							metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 2)
						} else if strings.HasSuffix(metricVal, "Gi") {
							strings.TrimSuffix(metricVal, "Gi")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
							metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 3)
						} else if strings.HasSuffix(metricVal, "Ti") {
							strings.TrimSuffix(metricVal, "Ti")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
							metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 4)
						} else if strings.HasSuffix(metricVal, "Pi") {
							strings.TrimSuffix(metricVal, "Pi")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 5)
						} else if strings.HasSuffix(metricVal, "Ei") {
							strings.TrimSuffix(metricVal, "Ei")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
							metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 6)
						} else if strings.HasSuffix(metricVal, "Zi") {
							strings.TrimSuffix(metricVal, "Zi")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 7)
						} else if strings.HasSuffix(metricVal, "Yi") {
							strings.TrimSuffix(metricVal, "Yi")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1024.0
							metricValue = math.Pow(metricValue, 8)
						} else if strings.HasSuffix(metricVal, "K") {
							strings.TrimSuffix(metricVal, "K")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 1)
						} else if strings.HasSuffix(metricVal, "M") {
							strings.TrimSuffix(metricVal, "M")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 2)
						} else if strings.HasSuffix(metricVal, "G") {
							strings.TrimSuffix(metricVal, "G")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 3)
						} else if strings.HasSuffix(metricVal, "T") {
							strings.TrimSuffix(metricVal, "T")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 4)
						} else if strings.HasSuffix(metricVal, "P") {
							strings.TrimSuffix(metricVal, "P")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 5)
						} else if strings.HasSuffix(metricVal, "E") {
							strings.TrimSuffix(metricVal, "E")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 6)
						} else if strings.HasSuffix(metricVal, "Z") {
							strings.TrimSuffix(metricVal, "Z")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 7)
						} else if strings.HasSuffix(metricVal, "Y") {
							strings.TrimSuffix(metricVal, "Y")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 8)
						} else {
							//assuming there are no units specified, it is bytes (the below conversion will fail for other unsupported 'units')
							metricValue, err := strconv.ParseFloat(metricVal, 64)
						}
		//convert to nanocores for cpu
		/*https://kubernetes.io/docs/tasks/configure-pod-container/assign-cpu-resource/ */
		case "cpu" : if strings.HasSuffix(metricVal, "m") {
							strings.TrimSuffix(metricVal, "m")
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64) 
                            metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 2)
						} else {
							//assuming no units specified, it is cores that we are converting to nanocores (the below conversion will fail for other unsupported 'units')
							metricValueFloat, err := strconv.ParseFloat(metricVal, 64)
							metricValue := metricValueFloat * 1000.0
							metricValue = math.Pow(metricValue, 3)
						}
		default :	metricValue = 0
		// @Log.warn("getMetricNumericValue: Unsupported metric #{metricName}. Returning 0 for metric value")
	}
	return metricValue
}

func getContainerResourceRequestsAndLimits(pods *v1.PodList, metricCategory string, metricNameToCollect string, metricNametoReturn string) []DataItem{
	var metricItems []DataItem
	clusterId := getClusterId()
	for _, pod := range pods.Items {
		var metricItem []DataItem
		
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
				//if container.Resources[metricCategory] != nil && container.Resources[metricCategory][metricNameToCollect] != nil {
				switch metricNameToCollect {
					case "cpu": if metricCategory == "limits" {
									if container.Resources.Limits.Cpu != nil {
										//Results returned in cores. Converting them to nanocores
										containerMetricValue := container.Resources.Limits.Cpu().MilliValue()
										metricValueMultiplier := math.Pow(1000, 2)
										//containerMetricValueInt, err := containerMetricValue.i.AsInt64()
										metricValue := float64(containerMetricValue) * metricValueMultiplier
									} else {
										if NodeMetrics.NodeMetricsHashKey != nil {
											containerMetricValue := NodeMetrics[nodeMetricsHashKey]
										}
									}
								} else if metricCategory == "requests" {
									containerMetricValue := container.Resources.Requests.Cpu.MilliValue()
									metricValueMultiplier := math.Pow(1000, 2)
									metricValue := float64(containerMetricValue) * metricValueMultiplier
								}
					case "memory": if metricCategory == "limits" {
										metricValue := container.Resources.Limits.Memory()
									} else if metricCategory == "requests" {
										metricValue := container.Resources.Requests.Memory()
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
				counters, err := json.Marshal(metricCounter)
				if (err == nil) {
					counterString := "[" + string(counters) + "]"
					record["Collections"] = counterString
				}
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
		var metricItem []DataItem
		nodeMetaData := node.ObjectMeta
		nodeName := nodeMetaData.Name
		if node.Status.MetricCategory != nil {
			metricValue := getMetricNumericValue(metricNameToCollect, node.Status.MetricCategory.MetricNameToCollect)
			record := make(map[interface{}]interface{})
			record["Timestamp"] = metricTime
			record["Host"] = nodeName
			record["ObjectName"] = "K8SNode"
			record["InstanceName"] = clusterId + "/" + nodeName
			metricCounter := MetricCollection {
				CounterName: metricNametoReturn,
				Value: metricValue,
			}
			counters, err := json.Marshal(metricCounter)
			if (err == nil) {
				counterString := "[" + string(counters) + "]"
				record["Collections"] = counterString
			}
			mapstructure.Decode(record, &metricItem)
			metricItems = append(metricItems, metricItem)
			//push node level metrics to a inmem hash so that we can use it looking up at container level.
			//Currently if container level cpu & memory limits are not defined we default to node level limits
			NodeMetrics[clusterId + "/" + nodeName + "_" + metricCategory + "_" + metricNameToCollect] = metricValue
			//@Log.info ("Node metric hash: #{@@NodeMetrics}")
		}
	}
	return metricItems
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
	fmt.Println("Getting pods...")
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Done getting pods...")
	var dataItems []DataItem
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "requests", "cpu","cpuRequestNanoCores"))
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "requests", "memory","memoryRequestBytes"))
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "limits", "cpu","cpuLimitNanoCores"))
	dataItems = append(dataItems, getContainerResourceRequestsAndLimits(pods, "limits", "memory","memoryLimitBytes"))

	fmt.Println("Getting nodes...")

	nodes, err := clientset.CoreV1().Nodes("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Done getting nodes...")
	dataItems = append(dataItems, parseNodeLimits(nodes, "allocatable", "cpu", "cpuAllocatableNanoCores"))
	dataItems = append(dataItems, parseNodeLimits(nodes, "allocatable", "memory", "memoryAllocatableBytes"))
	dataItems = append(dataItems, parseNodeLimits(nodes, "capacity", "cpu", "cpuCapacityNanoCores"))
	dataItems = append(dataItems, parseNodeLimits(nodes, "capacity", "memory", "memoryCapacityBytes"))

	podEntry := KubePodInventoryBlob{
		DataType:  "LINUX_PERF_BLOB",
		IPName:    "LogManagement",
		DataItems: dataItems,
	}

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








func parseAndEmitRecords(pods *v1.PodList, services *v1.ServiceList) {
	var dataItems []DataItem
	for _, pod := range pods.Items {
		currentTime := time.Now()
		batchTime := currentTime.UTC().Format(time.RFC3339)
		record := make(map[interface{}]interface{})
		var dataItem DataItem

		//This is the time that is mapped to become TimeGenerated
		record["CollectionTime"] = batchTime 
		podmetadata := pod.ObjectMeta
		record["Name"] = podmetadata.Name + "-rashmirsmemory"
		podNameSpace := podmetadata.Namespace
		record["Namespace"] = podNameSpace
		podLabels, err := json.Marshal(podmetadata.Labels)
		if (err == nil) {
			podLabelString := "[" + string(podLabels) + "]"
			record["PodLabel"] = podLabelString
		}
		//fmt.Println(podmetadata.Labels)
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
				}
				//TODO : Remove ContainerCreationTimeStamp from here since we are sending it as a metric
				//Picking up both container and node start time from cAdvisor to be consistent
				podRestartCount += containerRestartCount

				mapstructure.Decode(record, &dataItem)
				dataItems = append(dataItems, dataItem)
			} 
			} else {
				mapstructure.Decode(record, &dataItem)
				dataItems = append(dataItems, dataItem)
			}

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
	fmt.Println("Starting the application...")
	enumerate()
	fmt.Println("Terminating")

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}

