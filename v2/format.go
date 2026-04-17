package mtl_snowflake

// ReadableFormat 可读格式枚举
type ReadableFormat int

const (
	// FormatYYMMDD 年月日格式（2位年），总长度21位
	// 结构: YYMMDD(6位) + 编码后缀(15位)
	FormatYYMMDD ReadableFormat = 0

	// FormatYYYYMMDD 年月日格式（4位年），总长度23位
	// 结构: YYYYMMDD(8位) + 编码后缀(15位)
	FormatYYYYMMDD ReadableFormat = 1

	// FormatYYMMDDHHMMSS 年月日时分秒格式（2位年），总长度22位
	// 结构: YYMMDDHHMMSS(12位) + 编码后缀(10位)
	FormatYYMMDDHHMMSS ReadableFormat = 2

	// FormatYYYYMMDDHHMMSS 年月日时分秒格式（4位年），总长度24位
	// 结构: YYYYMMDDHHMMSS(14位) + 编码后缀(10位)
	FormatYYYYMMDDHHMMSS ReadableFormat = 3
)

// 编码乘数常量（用于合并编码算法）
const (
	// M: 机器ID × 时间线 × 序列号的最大值
	// 512 × 2 × 4096 = 4,194,304
	encodeMultiplierM = 4194304

	// S1: 时间线 × 序列号的最大值
	// 2 × 4096 = 8,192
	encodeMultiplierS1 = 8192

	// S2: 序列号的最大值
	// 4096 = 4,096
	encodeMultiplierS2 = 4096

	// 序列号最大值（用于取模）
	seqMaxForEncode = 4096

	// 机器ID最大值（用于取模）
	machineIDMaxForEncode = 512

	// 时间线最大值（用于取模）
	timelineMaxForEncode = 2
)