package common

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// NodeInfo 自定义一个结构体，只保留你业务真正需要的字段
type NodeInfo struct {
	OSName    string // 操作系统名称 (如: ubuntu 22.04)
	MachineID string // 机器唯一UUID
	CPUCore   int32  // CPU 逻辑核数(线程数)
	MemTotal  uint64 // 物理内存总大小 (单位: Bytes)
	DiskTotal uint64 // 根目录磁盘总大小 (单位: Bytes)
}

func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		fmt.Printf("get local addr err:%v\n", err)
		return ""
	} else {
		localIp := strings.Split(conn.LocalAddr().String(), ":")[0]
		_ = conn.Close()
		return localIp
	}
}

func GetHostName() string {
	nodeName := os.Getenv("MY_NODE_NAME")
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}
	return nodeName

}

//	func GetNodeInfo() sysinfo.SysInfo {
//		var si sysinfo.SysInfo
//		si.GetSysInfo()
//		return si
//	}
func GetNodeInfo() *NodeInfo {
	info := &NodeInfo{}

	// 1. 获取主机信息 (OS名称, SN)
	if h, err := host.Info(); err == nil {
		info.OSName = h.Platform + " " + h.PlatformVersion
		info.MachineID = h.HostID
	}

	// 2. 获取 CPU 逻辑核数 (Threads)
	if cores, err := cpu.Counts(true); err == nil {
		info.CPUCore = int32(cores)
	}

	// 3. 获取物理内存总容量 (Bytes)
	if v, err := mem.VirtualMemory(); err == nil {
		info.MemTotal = v.Total
	}

	// 4. 获取磁盘总容量 (Bytes)
	// 优先尝试获取根目录 "/", 如果失败(例如在Windows下)则尝试 "C:"
	d, err := disk.Usage("/")
	if err != nil {
		d, _ = disk.Usage("C:")
	}
	if d != nil {
		info.DiskTotal = d.Total
	}

	return info
}

func WriteFileWithString(filePath string, content string) error {
	return os.WriteFile(filePath, []byte(content), 0644)
}

func ReadFile(path string) (string, error) {
	res, err := os.ReadFile(path)
	return string(res), err
}

func GetDayAgoDate(num int) string {
	return time.Now().Add(time.Duration(num) * time.Hour * 24).Format("2006-01-02")
}

func GenJsonString(str string) string {
	data, _ := json.Marshal(str)
	return string(data)
}

func GenKvStringByMap(m map[string]string) string {
	res := ""
	for k, v := range m {
		res = fmt.Sprintf("%s %s=%s", res, k, v)
	}
	return res
}

func GentStringArrayByMap(m map[string]string) []string {
	res := []string{}
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return res
}
