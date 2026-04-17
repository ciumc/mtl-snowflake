package mtl_snowflake

import "errors"

// ErrTimeBackwardTooFar 时钟回退到基准时间之前
var ErrTimeBackwardTooFar = errors.New("time backward exceeds epoch, check server clock or set an earlier epoch")

// ErrNoAvailableTimeline 无可用时间线
var ErrNoAvailableTimeline = errors.New("no available timeline, clock backward too frequent, adjust clock sync strategy or increase timeline count")

// ErrMachineIDOverflow 机器ID超出范围
var ErrMachineIDOverflow = errors.New("machineID exceeds maximum value")

// ErrInvalidSettings 参数配置无效
var ErrInvalidSettings = errors.New("invalid settings: TimeBit + MachineIDBit + TimelineBit + SeqBit must equal 63")

// ErrEpochTooLate 基准时间晚于当前时间
var ErrEpochTooLate = errors.New("epoch must not be later than current time")

// ErrTimeOffsetExceeded 时间偏移超出最大限制
var ErrTimeOffsetExceeded = errors.New("current time offset exceeds maximum limit, increase TimeBit or set a later epoch")

// ErrPodNameNotSet POD_NAME 环境变量未设置
var ErrPodNameNotSet = errors.New("POD_NAME not set")

// ErrInvalidPodName Pod 名称格式无效
var ErrInvalidPodName = errors.New("invalid Pod name format")

// ErrInvalidOrdinal Pod 序号无效
var ErrInvalidOrdinal = errors.New("invalid ordinal")

// ErrInvalidReadableFormat 可读格式字符串无效
var ErrInvalidReadableFormat = errors.New("invalid readable format: length mismatch")

// ErrInvalidReadableValue 可读格式编码值超出范围
var ErrInvalidReadableValue = errors.New("invalid readable value: exceeds maximum limit")

// ErrYearOutOfRange 2位年份超出有效范围（2025-2094）
var ErrYearOutOfRange = errors.New("year out of valid range: must be between 25-94")

// ErrIncompatibleSettings 配置与可读格式编码不兼容
var ErrIncompatibleSettings = errors.New("incompatible settings: ToReadable/FromReadable requires MachineIDBit=9, TimelineBit=1, SeqBit=12")