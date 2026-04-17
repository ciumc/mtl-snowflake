package mtl_snowflake

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// GetMachineID 通过机器指纹生成 MachineID
// 指纹组成：hostname + MAC + IP
func GetMachineID(machineIDBit uint64) (int64, error) {
	// 收集机器指纹
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	mac := getFirstPhysicalMAC()
	ip := getFirstNonLoopbackIP()

	// 组合并哈希
	seed := fmt.Sprintf("%s|%s|%s", hostname, mac, ip)
	hash := sha256.Sum256([]byte(seed))

	// 映射到 MachineIDBit 范围
	value := int64(binary.BigEndian.Uint64(hash[:8]))
	maxMachineID := int64(1 << machineIDBit) - 1
	machineID := value & maxMachineID

	return machineID, nil
}

// getFirstNonLoopbackIP 获取首个非回环 IP
func getFirstNonLoopbackIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// getFirstPhysicalMAC 获取首个物理网卡 MAC（优化：提前终止）
func getFirstPhysicalMAC() string {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		// 快速排除回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// 排除常见虚拟网卡前缀（veth, docker, br-, lo）
		name := iface.Name
		if len(name) > 0 {
			if strings.HasPrefix(name, "veth") ||
				strings.HasPrefix(name, "docker") ||
				strings.HasPrefix(name, "br-") ||
				strings.HasPrefix(name, "lo") {
				continue
			}
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return "00:00:00:00:00:00"
}

// GetMachineIDFromPodName 从 K8s StatefulSet Pod 名称提取序号
func GetMachineIDFromPodName(machineIDBit uint64) (int64, error) {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return 0, ErrPodNameNotSet
	}

	// StatefulSet 格式: {name}-{ordinal}
	parts := strings.Split(podName, "-")
	if len(parts) < 2 {
		return 0, ErrInvalidPodName
	}

	ordinal, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0, ErrInvalidOrdinal
	}

	maxMachineID := int64(1 << machineIDBit) - 1
	if ordinal < 0 || ordinal > maxMachineID {
		return 0, ErrMachineIDOverflow
	}

	return ordinal, nil
}