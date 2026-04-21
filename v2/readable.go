package mtl_snowflake

import (
	"time"
)

// ToReadable 将ID转换为可读格式（默认使用 FormatYYYYMMDDHHMMSS）
func (g *IDGenerator) ToReadable(id int64) (string, error) {
	return g.ToReadableWithFormat(id, FormatYYYYMMDDHHMMSS)
}

// ToReadableWithFormat 将ID转换为可读格式（零分配优化版本）
// format 指定时间前缀精度，返回固定长度的数字字符串
// 输出时间使用系统本地时区
// 注意：此方法仅支持默认配置 (MachineIDBit=9, TimelineBit=1, SeqBit=12)
func (g *IDGenerator) ToReadableWithFormat(id int64, format ReadableFormat) (string, error) {
	s := g.settings

	// 验证配置兼容性（编码常量仅支持默认配置）
	if !g.isReadableCompatible() {
		return "", ErrIncompatibleSettings
	}

	// 解析ID各部分
	info := g.Decompose(id)

	// 计算时间
	timePart := info.Timestamp
	msOffset := s.Epoch/1e6 + timePart

	// 使用本地时区
	t := time.UnixMilli(msOffset).Local()
	year := t.Year()
	month := int(t.Month())
	day := t.Day()
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()
	ms := msOffset % 1000

	// 使用固定大小 buffer，避免动态分配
	bufLen := expectedLength(format)
	var buf [24]byte // 最大24位，足够容纳所有格式
	pos := 0

	switch format {
	case FormatYYMMDD:
		// YYMMDD (6位)
		writeInt2(&buf, &pos, year%100)
		writeInt2(&buf, &pos, month)
		writeInt2(&buf, &pos, day)
		// 编码后缀 (15位)
		timeInDayMs := int64(hour)*3600000 + int64(minute)*60000 + int64(second)*1000 + ms
		encodedSuffix := encodeFull(timeInDayMs, info.MachineID, info.Timeline, info.Sequence)
		writeInt15(&buf, &pos, encodedSuffix)

	case FormatYYYYMMDD:
		// YYYYMMDD (8位)
		writeInt4(&buf, &pos, year)
		writeInt2(&buf, &pos, month)
		writeInt2(&buf, &pos, day)
		// 编码后缀 (15位)
		timeInDayMs := int64(hour)*3600000 + int64(minute)*60000 + int64(second)*1000 + ms
		encodedSuffix := encodeFull(timeInDayMs, info.MachineID, info.Timeline, info.Sequence)
		writeInt15(&buf, &pos, encodedSuffix)

	case FormatYYMMDDHHMMSS:
		// YYMMDDHHMMSS (12位)
		writeInt2(&buf, &pos, year%100)
		writeInt2(&buf, &pos, month)
		writeInt2(&buf, &pos, day)
		writeInt2(&buf, &pos, hour)
		writeInt2(&buf, &pos, minute)
		writeInt2(&buf, &pos, second)
		// 编码后缀 (10位)
		encodedSuffix := encodeShort(ms, info.MachineID, info.Timeline, info.Sequence)
		writeInt10(&buf, &pos, encodedSuffix)

	case FormatYYYYMMDDHHMMSS:
		// YYYYMMDDHHMMSS (14位)
		writeInt4(&buf, &pos, year)
		writeInt2(&buf, &pos, month)
		writeInt2(&buf, &pos, day)
		writeInt2(&buf, &pos, hour)
		writeInt2(&buf, &pos, minute)
		writeInt2(&buf, &pos, second)
		// 编码后缀 (10位)
		encodedSuffix := encodeShort(ms, info.MachineID, info.Timeline, info.Sequence)
		writeInt10(&buf, &pos, encodedSuffix)
	}

	return string(buf[:bufLen]), nil
}

// writeInt2 写入2位数字（补零）
func writeInt2(buf *[24]byte, pos *int, n int) {
	buf[*pos] = byte('0' + n/10)
	buf[*pos+1] = byte('0' + n%10)
	*pos += 2
}

// writeInt4 写入4位数字（补零）
func writeInt4(buf *[24]byte, pos *int, n int) {
	buf[*pos] = byte('0' + n/1000)
	buf[*pos+1] = byte('0' + (n/100)%10)
	buf[*pos+2] = byte('0' + (n/10)%10)
	buf[*pos+3] = byte('0' + n%10)
	*pos += 4
}

// writeInt10 写入10位数字（补零）
func writeInt10(buf *[24]byte, pos *int, n int64) {
	for i := 9; i >= 0; i-- {
		buf[*pos+i] = byte('0' + n%10)
		n /= 10
	}
	*pos += 10
}

// writeInt15 写入15位数字（补零）
func writeInt15(buf *[24]byte, pos *int, n int64) {
	for i := 14; i >= 0; i-- {
		buf[*pos+i] = byte('0' + n%10)
		n /= 10
	}
	*pos += 15
}

// encodeFull 编码: 时内毫秒 + 机器ID + 时间线 + 序号
func encodeFull(timeInDayMs, machineID, timeline, seq int64) int64 {
	return timeInDayMs*encodeMultiplierM + machineID*encodeMultiplierS1 + timeline*encodeMultiplierS2 + seq
}

// encodeShort 编码: 毫秒 + 机器ID + 时间线 + 序号
func encodeShort(ms, machineID, timeline, seq int64) int64 {
	return ms*encodeMultiplierM + machineID*encodeMultiplierS1 + timeline*encodeMultiplierS2 + seq
}

// suffixLength 返回后缀长度
func suffixLength(format ReadableFormat) int {
	switch format {
	case FormatYYMMDD, FormatYYYYMMDD:
		return 15
	case FormatYYMMDDHHMMSS, FormatYYYYMMDDHHMMSS:
		return 10
	default:
		return 15
	}
}

// FromReadable 从可读格式还原原始ID
// 注意：此方法仅支持默认配置 (MachineIDBit=9, TimelineBit=1, SeqBit=12)
func (g *IDGenerator) FromReadable(readable string, format ReadableFormat) (int64, error) {
	// 验证配置兼容性
	if !g.isReadableCompatible() {
		return 0, ErrIncompatibleSettings
	}

	s := g.settings

	// 验证长度
	expectedLen := expectedLength(format)
	if len(readable) != expectedLen {
		return 0, ErrInvalidReadableFormat
	}

	// 验证纯数字
	for _, c := range readable {
		if c < '0' || c > '9' {
			return 0, ErrInvalidReadableFormat
		}
	}

	// 提取第一段和第二段
	prefixLen := prefixLength(format)
	prefix := readable[:prefixLen]
	suffix := readable[prefixLen:]

	// 手动解析后缀编码值（避免 strconv.ParseInt）
	encodedSuffix := parseSuffix(suffix)

	// 解码后缀
	var timeInDayMs int64
	var ms int64
	var machineID int64
	var timeline int64
	var seq int64

	switch format {
	case FormatYYMMDD, FormatYYYYMMDD:
		seq = encodedSuffix % seqMaxForEncode
		encodedSuffix /= seqMaxForEncode
		timeline = encodedSuffix % timelineMaxForEncode
		encodedSuffix /= timelineMaxForEncode
		machineID = encodedSuffix % machineIDMaxForEncode
		timeInDayMs = encodedSuffix / machineIDMaxForEncode

		if timeInDayMs > 86399999 {
			return 0, ErrInvalidReadableValue
		}

	case FormatYYMMDDHHMMSS, FormatYYYYMMDDHHMMSS:
		seq = encodedSuffix % seqMaxForEncode
		encodedSuffix /= seqMaxForEncode
		timeline = encodedSuffix % timelineMaxForEncode
		encodedSuffix /= timelineMaxForEncode
		machineID = encodedSuffix % machineIDMaxForEncode
		ms = encodedSuffix / machineIDMaxForEncode

		if ms > 999 {
			return 0, ErrInvalidReadableValue
		}
	}

	// 验证机器ID范围
	if machineID > s.machineIDMax {
		return 0, ErrInvalidReadableValue
	}

	// 解析时间前缀
	year, month, day, hour, minute, second, err := parsePrefix(prefix, format)
	if err != nil {
		return 0, err
	}

	// 计算时间戳偏移（使用本地时区）
	var timestamp int64
	switch format {
	case FormatYYMMDD, FormatYYYYMMDD:
		hour = int(timeInDayMs / 3600000)
		minute = int((timeInDayMs % 3600000) / 60000)
		second = int((timeInDayMs % 60000) / 1000)
		ms = timeInDayMs % 1000

		t := time.Date(year, time.Month(month), day, hour, minute, second, int(ms*1e6), time.Local)
		timestamp = (t.UnixNano() - s.Epoch) / 1e6

	case FormatYYMMDDHHMMSS, FormatYYYYMMDDHHMMSS:
		t := time.Date(year, time.Month(month), day, hour, minute, second, int(ms*1e6), time.Local)
		timestamp = (t.UnixNano() - s.Epoch) / 1e6
	}

	// 组装原始ID
	id := (timestamp << s.timeShift) |
		(machineID << s.machineShift) |
		(timeline << s.timelineShift) |
		seq

	return id, nil
}

// expectedLength 返回预期总长度
func expectedLength(format ReadableFormat) int {
	switch format {
	case FormatYYMMDD:
		return 21
	case FormatYYYYMMDD:
		return 23
	case FormatYYMMDDHHMMSS:
		return 22
	case FormatYYYYMMDDHHMMSS:
		return 24
	default:
		return 21
	}
}

// prefixLength 返回前缀长度
func prefixLength(format ReadableFormat) int {
	switch format {
	case FormatYYMMDD:
		return 6
	case FormatYYYYMMDD:
		return 8
	case FormatYYMMDDHHMMSS:
		return 12
	case FormatYYYYMMDDHHMMSS:
		return 14
	default:
		return 6
	}
}

// parsePrefix 解析时间前缀（手动解析，避免 strconv）
func parsePrefix(prefix string, format ReadableFormat) (year, month, day, hour, minute, second int, err error) {
	switch format {
	case FormatYYMMDD:
		yy := parse2Digits(prefix[0:2])
		if yy < 25 || yy > 94 {
			return 0, 0, 0, 0, 0, 0, ErrYearOutOfRange
		}
		year = 2000 + yy
		month = parse2Digits(prefix[2:4])
		day = parse2Digits(prefix[4:6])

	case FormatYYYYMMDD:
		year = parse4Digits(prefix[0:4])
		month = parse2Digits(prefix[4:6])
		day = parse2Digits(prefix[6:8])

	case FormatYYMMDDHHMMSS:
		yy := parse2Digits(prefix[0:2])
		if yy < 25 || yy > 94 {
			return 0, 0, 0, 0, 0, 0, ErrYearOutOfRange
		}
		year = 2000 + yy
		month = parse2Digits(prefix[2:4])
		day = parse2Digits(prefix[4:6])
		hour = parse2Digits(prefix[6:8])
		minute = parse2Digits(prefix[8:10])
		second = parse2Digits(prefix[10:12])

	case FormatYYYYMMDDHHMMSS:
		year = parse4Digits(prefix[0:4])
		month = parse2Digits(prefix[4:6])
		day = parse2Digits(prefix[6:8])
		hour = parse2Digits(prefix[8:10])
		minute = parse2Digits(prefix[10:12])
		second = parse2Digits(prefix[12:14])
	}

	if month < 1 || month > 12 || day < 1 || day > 31 ||
		hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, 0, 0, 0, 0, 0, ErrInvalidReadableFormat
	}

	return year, month, day, hour, minute, second, nil
}

// parse2Digits 手动解析2位数字
func parse2Digits(s string) int {
	return int(s[0]-'0')*10 + int(s[1]-'0')
}

// parse4Digits 手动解析4位数字
func parse4Digits(s string) int {
	return int(s[0]-'0')*1000 + int(s[1]-'0')*100 + int(s[2]-'0')*10 + int(s[3]-'0')
}

// parseSuffix 手动解析后缀数字（10位或15位）
func parseSuffix(s string) int64 {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n
}

// isReadableCompatible 检查配置是否与可读格式编码兼容
// 编码常量仅支持默认配置: MachineIDBit=9, TimelineBit=1, SeqBit=12
func (g *IDGenerator) isReadableCompatible() bool {
	s := g.settings
	return s.MachineIDBit == 9 && s.TimelineBit == 1 && s.SeqBit == 12
}