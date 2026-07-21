package view_server

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type OnePod struct {
	Namespace      string `json:"namespace"`
	Image          string `json:"image"`
	Ip             string `json:"ip"`
	Restarts       int    `json:"restarts"`
	Status         string `json:"status"`
	Ready          string `json:"ready"`
	Name           string `json:"name"`
	CpuRequest     string `json:"cpuRequest"`
	CpuRequestInfo string `json:"cpuRequestInfo"`
	CpuLimit       string `json:"cpuLimit"`
	MemRequest     string `json:"memRequest"`
	MemRequestInfo string `json:"memRequestInfo"`
	MemLimit       string `json:"memLimit"`
	Age            string `json:"age"`
}

func podConvert(pod *corev1.Pod) *OnePod {
	if pod == nil {
		return nil
	}

	var res OnePod
	res.Namespace = pod.Namespace
	res.Name = pod.Name
	res.Age = translateTimestampSince(pod.CreationTimestamp)

	status := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		status = pod.Status.Reason
	}
	res.Status = status

	restarts := 0
	readyContainers := 0
	totalContainers := len(pod.Spec.Containers)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Ready && containerStatus.State.Running != nil {
			readyContainers++
		}
		restarts += int(containerStatus.RestartCount)
	}
	res.Ready = fmt.Sprintf("%d/%d", readyContainers, totalContainers)
	res.Ip = pod.Status.PodIP
	res.Restarts = restarts

	var images []string
	var totalCpuReq, totalCpuLim, totalMemReq, totalMemLim int64

	for _, c := range pod.Spec.Containers {
		if c.Image != "" {
			images = append(images, c.Image)
		}
		totalCpuReq += c.Resources.Requests.Cpu().MilliValue()
		totalCpuLim += c.Resources.Limits.Cpu().MilliValue()
		totalMemReq += c.Resources.Requests.Memory().Value()
		totalMemLim += c.Resources.Limits.Memory().Value()
	}

	if len(pod.Spec.Containers) > 0 {
		res.Image = pod.Spec.Containers[0].Image
		if len(images) > 1 {
			res.Image = fmt.Sprintf("%s (+%d containers)", images[0], len(images)-1)
		}
	}

	if totalCpuReq > 0 {
		res.CpuRequest = fmt.Sprintf("%.2f核", float64(totalCpuReq)/1000)
	} else {
		res.CpuRequest = "0"
	}
	if totalCpuLim > 0 {
		res.CpuLimit = fmt.Sprintf("%.2f核", float64(totalCpuLim)/1000)
	} else {
		res.CpuLimit = "0"
	}

	const mib = 1024 * 1024
	if totalMemReq > 0 {
		res.MemRequest = fmt.Sprintf("%.2f MiB", float64(totalMemReq)/float64(mib))
	} else {
		res.MemRequest = "0"
	}
	if totalMemLim > 0 {
		res.MemLimit = fmt.Sprintf("%.2f MiB", float64(totalMemLim)/float64(mib))
	} else {
		res.MemLimit = "0"
	}

	res.CpuRequestInfo = fmt.Sprintf("%s / %s", res.CpuRequest, res.CpuLimit)
	res.MemRequestInfo = fmt.Sprintf("%s / %s", res.MemRequest, res.MemLimit)

	return &res
}
