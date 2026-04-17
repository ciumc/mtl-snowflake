package mtl_snowflake

import (
	"time"
)

// Settings ID生成器参数配置
type Settings struct {
	TimeBit      uint64 // 时间戳位数
	MachineIDBit uint64 // 机器ID位数
	TimelineBit  uint64 // 时间线位数
	SeqBit       uint64 // 序列号位数
	Epoch        int64  // 基准时间（纳秒）

	// 预计算字段（内部使用，初始化后自动填充）
	timeMax        int64  // 时间戳最大值 (2^TimeBit - 1)
	machineIDMax   int64  // 机器ID最大值 (2^MachineIDBit - 1)
	timelineCount  int64  // 时间线数量 (2^TimelineBit)
	timelineMax    int64  // 时间线最大值 (2^TimelineBit - 1)
	seqMax         int64  // 序列号最大值 (2^SeqBit - 1)
	seqMask        int64  // 序列号掩码，用于快速循环
	timeShift      uint64 // 时间戳左移位数
	machineShift   uint64 // 机器ID左移位数
	timelineShift  uint64 // 时间线左移位数
	inTimeMax      int64  // 时间内最大值（MachineID + Timeline + Seq）
}

// DefaultEpoch 默认基准时间 2025-01-01 00:00:00 UTC
var DefaultEpoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

// DefaultSettings 返回默认配置
func DefaultSettings() *Settings {
	return &Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        DefaultEpoch,
	}
}

// Validate 校验配置参数
func (s *Settings) Validate() error {
	totalBits := s.TimeBit + s.MachineIDBit + s.TimelineBit + s.SeqBit + 1
	if totalBits != 64 {
		return ErrInvalidSettings
	}
	s.initPresets()
	return nil
}

// ValidateWithTime 校验配置参数并检查时间有效性
func (s *Settings) ValidateWithTime() error {
	if err := s.Validate(); err != nil {
		return err
	}

	// 检查基准时间不晚于当前时间
	curTime := (time.Now().UnixNano() - s.Epoch) / 1e6
	if curTime < 0 {
		return ErrEpochTooLate
	}

	// 检查当前时间不超过最大偏移
	if curTime > s.timeMax {
		return ErrTimeOffsetExceeded
	}

	return nil
}

// initPresets 预计算参数
func (s *Settings) initPresets() {
	s.timeMax = (1 << s.TimeBit) - 1
	s.machineIDMax = (1 << s.MachineIDBit) - 1
	s.timelineCount = 1 << s.TimelineBit
	s.timelineMax = (1 << s.TimelineBit) - 1
	s.seqMax = (1 << s.SeqBit) - 1
	s.seqMask = s.seqMax // 用于位运算循环

	s.timeShift = s.MachineIDBit + s.TimelineBit + s.SeqBit
	s.machineShift = s.TimelineBit + s.SeqBit
	s.timelineShift = s.SeqBit

	// 预计算 ToReadable 参数
	s.inTimeMax = (1 << s.timeShift) - 1
}