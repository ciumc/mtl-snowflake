package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	mtl "github.com/ciumc/mtl-snowflake/v2"
)

func main() {
	fmt.Println("=== mtl-snowflake 使用示例 ===")
	fmt.Println()

	// 示例1：基本使用
	basicExample()

	// 示例2：自定义配置
	customSettingsExample()

	// 示例3：集群部署
	clusterExample()

	// 示例4：并发使用
	concurrentExample()

	// 示例5：自动获取MachineID
	machineIDExample()

	// 示例6：ID解析
	decomposeExample()

	// 示例7：错误处理
	errorHandlingExample()

	// 示例8：生产环境推荐用法
	productionExample()
}

// 示例1：基本使用
func basicExample() {
	fmt.Println("【示例1：基本使用】")
	fmt.Println("使用默认配置创建ID生成器：")
	fmt.Println("  TimeBit=41 (69年), MachineIDBit=9 (512节点), TimelineBit=1 (2条时间线), SeqBit=12 (4096/ms)")

	machineID := int64(0)
	idGen, err := mtl.NewGenerator(machineID)
	if err != nil {
		fmt.Printf("  创建失败: %v\n", err)
		return
	}

	// 生成5个ID
	fmt.Println("  生成的ID:")
	for i := 0; i < 5; i++ {
		id, err := idGen.Generate()
		if err != nil {
			fmt.Printf("    生成失败: %v\n", err)
			continue
		}
		readable := idGen.ToReadableWithFormat(id, mtl.FormatYYYYMMDDHHMMSS)
		fmt.Printf("    ID=%d, 可读=%s\n", id, readable)
	}
	fmt.Println()
}

// 示例2：自定义配置
func customSettingsExample() {
	fmt.Println("【示例2：自定义配置】")

	// 高吞吐配置
	fmt.Println("  高吞吐配置 (MachineIDBit=6, SeqBit=15):")
	gen1, _ := mtl.NewGeneratorWithSettings(0, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 6,
		TimelineBit:  1,
		SeqBit:       15,
		Epoch:        mtl.DefaultEpoch,
	})
	id1, _ := gen1.Generate()
	fmt.Printf("    ID=%d\n", id1)

	// 高容错配置
	fmt.Println("  高容错配置 (TimelineBit=2, 4条时间线):")
	gen2, _ := mtl.NewGeneratorWithSettings(0, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 8,
		TimelineBit:  2,
		SeqBit:       12,
		Epoch:        mtl.DefaultEpoch,
	})
	id2, _ := gen2.Generate()
	fmt.Printf("    ID=%d, Timeline范围=[0,%d]\n", id2, (1<<2)-1)

	// 单时间线配置
	fmt.Println("  单时间线配置 (TimelineBit=0, 禁用切换):")
	gen3, _ := mtl.NewGeneratorWithSettings(0, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 10,
		TimelineBit:  0,
		SeqBit:       12,
		Epoch:        mtl.DefaultEpoch,
	})
	id3, _ := gen3.Generate()
	fmt.Printf("    ID=%d\n", id3)
	fmt.Println()
}

// 示例3：集群部署
func clusterExample() {
	fmt.Println("【示例3：集群部署】")
	fmt.Println("  不同节点使用不同MachineID:")

	gen1, _ := mtl.NewGenerator(0)
	gen2, _ := mtl.NewGenerator(1)
	gen3, _ := mtl.NewGenerator(2)

	id1, _ := gen1.Generate()
	id2, _ := gen2.Generate()
	id3, _ := gen3.Generate()

	info1 := gen1.Decompose(id1)
	info2 := gen2.Decompose(id2)
	info3 := gen3.Decompose(id3)

	fmt.Printf("    节点0: ID=%d, MachineID=%d\n", id1, info1.MachineID)
	fmt.Printf("    节点1: ID=%d, MachineID=%d\n", id2, info2.MachineID)
	fmt.Printf("    节点2: ID=%d, MachineID=%d\n", id3, info3.MachineID)
	fmt.Println()
}

// 示例4：并发使用
func concurrentExample() {
	fmt.Println("【示例4：并发使用】")
	fmt.Println("  10个goroutine并发各生成100个ID:")

	gen, _ := mtl.NewGenerator(0)
	var wg sync.WaitGroup
	ids := make(chan int64, 1000)
	start := time.Now()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				id, err := gen.Generate()
				if err != nil {
					fmt.Printf("    生成失败: %v\n", err)
					return
				}
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	elapsed := time.Since(start)

	// 验证唯一性
	unique := make(map[int64]bool)
	for id := range ids {
		unique[id] = true
	}

	fmt.Printf("    总生成: %d 个ID\n", len(unique))
	fmt.Printf("    耗时: %v\n", elapsed)
	fmt.Printf("    吞吐: %.2f ID/s\n", float64(len(unique))/elapsed.Seconds())
	fmt.Println()
}

// 示例5：自动获取MachineID
func machineIDExample() {
	fmt.Println("【示例5：自动获取MachineID】")

	machineIDBit := uint64(9)

	// 方式1：指纹哈希
	fmt.Println("  方式1: 指纹哈希 (hostname+MAC+IP)")
	id1, err := mtl.GetMachineID(machineIDBit)
	if err != nil {
		fmt.Printf("    获取失败: %v\n", err)
	} else {
		fmt.Printf("    MachineID=%d\n", id1)
	}

	// 方式2：K8s Pod名称
	fmt.Println("  方式2: K8s StatefulSet Pod序号")
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		fmt.Println("    POD_NAME未设置，模拟示例...")
		// 模拟设置
		os.Setenv("POD_NAME", "id-generator-5")
		id2, _ := mtl.GetMachineIDFromPodName(machineIDBit)
		fmt.Printf("    Pod=id-generator-5, MachineID=%d\n", id2)
		os.Unsetenv("POD_NAME")
	} else {
		id2, _ := mtl.GetMachineIDFromPodName(machineIDBit)
		fmt.Printf("    Pod=%s, MachineID=%d\n", podName, id2)
	}
	fmt.Println()
}

// 示例6：ID解析
func decomposeExample() {
	fmt.Println("【示例6：ID解析】")

	gen, err := mtl.NewGeneratorWithSettings(123, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        mtl.DefaultEpoch,
	})
	if err != nil {
		fmt.Printf("  创建失败: %v\n", err)
		return
	}

	id, err := gen.Generate()
	if err != nil {
		fmt.Printf("  生成失败: %v\n", err)
		return
	}
	info := gen.Decompose(id)
	readable := gen.ToReadableWithFormat(id, mtl.FormatYYYYMMDDHHMMSS)

	fmt.Printf("  ID: %d\n", id)
	fmt.Printf("  可读格式: %s\n", readable)
	fmt.Printf("  解析结果:\n")
	fmt.Printf("    Timestamp: %d (毫秒偏移)\n", info.Timestamp)
	fmt.Printf("    MachineID: %d\n", info.MachineID)
	fmt.Printf("    Timeline: %d\n", info.Timeline)
	fmt.Printf("    Sequence: %d\n", info.Sequence)

	// 计算实际时间
	actualTime := time.Unix(0, mtl.DefaultEpoch+info.Timestamp*1e6)
	fmt.Printf("  实际时间: %s\n", actualTime.Format("2006-01-02 15:04:05.000"))
	fmt.Println()
}

// 示例7：错误处理
func errorHandlingExample() {
	fmt.Println("【示例7：错误处理】")

	// MachineID超限
	fmt.Println("  MachineID超限:")
	_, err := mtl.NewGeneratorWithSettings(600, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        mtl.DefaultEpoch,
	})
	if errors.Is(err, mtl.ErrMachineIDOverflow) {
		fmt.Printf("    正确捕获错误: %v\n", err)
	}

	// 无效配置
	fmt.Println("  无效配置 (总位数不等于63):")
	_, err = mtl.NewGeneratorWithSettings(0, mtl.Settings{
		TimeBit:      41,
		MachineIDBit: 10, // 导致总位数=64而非63
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        mtl.DefaultEpoch,
	})
	if errors.Is(err, mtl.ErrInvalidSettings) {
		fmt.Printf("    正确捕获错误: %v\n", err)
	}

	// Pod名称未设置
	fmt.Println("  POD_NAME未设置:")
	os.Unsetenv("POD_NAME")
	_, err = mtl.GetMachineIDFromPodName(9)
	if err != nil {
		fmt.Printf("    正确捕获错误: %v\n", err)
	}
	fmt.Println()
}

// 示例8：生产环境推荐用法（GetMachineID + NewGenerator 组合）
func productionExample() {
	fmt.Println("【示例8：生产环境推荐用法】")
	fmt.Println("  物理机/VM部署：自动获取MachineID并创建生成器")

	// 1. 获取配置
	settings := mtl.DefaultSettings()

	// 2. 自动获取 MachineID（指纹哈希）
	machineID, err := mtl.GetMachineID(settings.MachineIDBit)
	if err != nil {
		fmt.Printf("    获取MachineID失败: %v\n", err)
		return
	}
	fmt.Printf("    自动获取 MachineID=%d (范围:0-%d)\n", machineID, (1<<settings.MachineIDBit)-1)

	// 3. 创建生成器
	idGen, err := mtl.NewGeneratorWithSettings(machineID, *settings)
	if err != nil {
		fmt.Printf("    创建生成器失败: %v\n", err)
		return
	}

	// 4. 生成ID
	fmt.Println("    生成的ID:")
	for i := 0; i < 3; i++ {
		id, err := idGen.Generate()
		if err != nil {
			fmt.Printf("      生成失败: %v\n", err)
			continue
		}
		info := idGen.Decompose(id)
		readable := idGen.ToReadableWithFormat(id, mtl.FormatYYYYMMDDHHMMSS)
		fmt.Printf("      ID=%d, 可读=%s, MachineID=%d\n", id, readable, info.MachineID)
	}
	fmt.Println()

	// K8s StatefulSet 部署示例
	fmt.Println("  K8s StatefulSet部署：零碰撞方案")
	fmt.Println("    配置YAML:")
	fmt.Println("      env:")
	fmt.Println("        - name: POD_NAME")
	fmt.Println("          valueFrom:")
	fmt.Println("            fieldRef:")
	fmt.Println("              fieldPath: metadata.name")
	fmt.Println()
	fmt.Println("    代码示例:")
	fmt.Println("      machineID, _ := mtl.GetMachineIDFromPodName(9)")
	fmt.Println("      idGen, _ := mtl.NewGenerator(machineID)")
	fmt.Println()
}
