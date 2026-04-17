// Package mtl_snowflake 提供基于多时间线改进的 Snowflake 分布式ID生成器。
//
// 通过时间线切换机制解决时钟回退问题，保证ID全局唯一。
// 默认配置：41bit时间戳(69年)，9bit机器ID(512节点)，1bit时间线(2条)，12bit序列号(4096/ms)
//
// 基本用法：
//
//	gen, _ := mtl_snowflake.NewGenerator(0)
//	id, _ := gen.Generate()
//	readable := gen.ToReadableWithFormat(id, mtl_snowflake.FormatYYYYMMDDHHMMSS)
package mtl_snowflake

import (
	"math/rand"
	"sync"
	"time"
)

const (
	timeUnit    = 1e6 // 毫秒（纳秒转毫秒）
	maxWaitTime = 1   // 最大等待时间（毫秒）
)

// IDGenerator 分布式ID生成器
type IDGenerator struct {
	mutex            sync.Mutex
	settings         *Settings
	timelineProgress []int64
	curTimeline      int64
	seq              int64
	machineID        int64
}

// NewGenerator 使用默认配置创建生成器
func NewGenerator(machineID int64) (*IDGenerator, error) {
	return NewGeneratorWithSettings(machineID, *DefaultSettings())
}

// NewGeneratorWithSettings 使用自定义配置创建生成器
func NewGeneratorWithSettings(machineID int64, settings Settings) (*IDGenerator, error) {
	// 参数校验（包含时间有效性检查）
	if err := settings.ValidateWithTime(); err != nil {
		return nil, err
	}

	if machineID < 0 || machineID > settings.machineIDMax {
		return nil, ErrMachineIDOverflow
	}

	// 初始化时间线进度
	timelineProgress := make([]int64, settings.timelineCount)

	// 随机选择初始时间线
	curTimeline := int64(0)
	if settings.timelineCount > 1 {
		curTimeline = rand.Int63n(settings.timelineCount)
	}

	return &IDGenerator{
		settings:         &settings,
		timelineProgress: timelineProgress,
		curTimeline:      curTimeline,
		machineID:        machineID,
	}, nil
}

// Generate 生成ID
func (g *IDGenerator) Generate() (int64, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	s := g.settings

	// 获取当前时间偏移
	curTime := (time.Now().UnixNano() - s.Epoch) / timeUnit
	progress := g.timelineProgress[g.curTimeline]

	// 处理时钟回退
	if curTime < progress {
		if curTime < 0 {
			return 0, ErrTimeBackwardTooFar
		}

		// 小幅回退：等待时间追回
		if progress - curTime <= maxWaitTime {
			time.Sleep(time.Duration(progress - curTime) * time.Millisecond)
			curTime = (time.Now().UnixNano() - s.Epoch) / timeUnit
		} else {
			// 大幅回退：切换时间线
			if s.timelineCount == 1 {
				return 0, ErrNoAvailableTimeline
			}

			// 查找可用时间线
			// 优化：当只有2条时间线时直接切换，无需遍历
			var foundTimeline int64 = -1
			if s.timelineCount == 2 {
				otherTimeline := 1 - g.curTimeline
				if g.timelineProgress[otherTimeline] < curTime {
					foundTimeline = otherTimeline
				}
			} else {
				// 多时间线：遍历查找最快的
				for i, p := range g.timelineProgress {
					if p < curTime {
						foundTimeline = int64(i)
						break // 找到第一个即可（都是初始化为0）
					}
				}
			}
			if foundTimeline == -1 {
				return 0, ErrNoAvailableTimeline
			}

			// 切换时间线
			g.timelineProgress[g.curTimeline] = curTime
			g.curTimeline = foundTimeline
		}
	}

	// 序列号处理
	// 同一毫秒：递增序列号；否则重置为0
	if curTime == progress {
		if g.seq = (g.seq + 1) & s.seqMask; g.seq == 0 {
			// 序列号用完，精确等待下一毫秒
			waitDuration := s.Epoch + (curTime+1)*timeUnit - time.Now().UnixNano()
			if waitDuration > 0 {
				time.Sleep(time.Duration(waitDuration))
			}
			curTime = (time.Now().UnixNano() - s.Epoch) / timeUnit
		}
	} else {
		g.seq = 0
	}

	// 时间线推进
	g.timelineProgress[g.curTimeline] = curTime

	// 组装ID
	return (curTime << s.timeShift) |
		(g.machineID << s.machineShift) |
		(g.curTimeline << s.timelineShift) |
		g.seq, nil
}

// IDInfo ID组成部分
type IDInfo struct {
	Timestamp int64
	MachineID int64
	Timeline  int64
	Sequence  int64
}

// Decompose 解析ID组成部分
func (g *IDGenerator) Decompose(id int64) IDInfo {
	s := g.settings
	return IDInfo{
		Sequence:  id & s.seqMax,
		Timeline:  (id >> s.timelineShift) & s.timelineMax,
		MachineID: (id >> s.machineShift) & s.machineIDMax,
		Timestamp: (id >> s.timeShift) & s.timeMax,
	}
}

// GetMachineID 获取机器ID
func (g *IDGenerator) GetMachineID() int64 {
	return g.machineID
}

// GetSettings 获取配置
func (g *IDGenerator) GetSettings() Settings {
	return *g.settings
}