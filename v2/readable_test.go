package mtl_snowflake

import (
	"testing"
)

func TestToReadableReversible(t *testing.T) {
	gen, err := NewGenerator(123)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	// 生成测试ID
	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 测试所有格式的可逆性
	formats := []ReadableFormat{
		FormatYYMMDD,
		FormatYYYYMMDD,
		FormatYYMMDDHHMMSS,
		FormatYYYYMMDDHHMMSS,
	}

	for _, format := range formats {
		readable := gen.ToReadableWithFormat(id, format)
		recoveredID, err := gen.FromReadable(readable, format)
		if err != nil {
			t.Errorf("FromReadable failed for format %d: %v, readable=%s", format, err, readable)
			continue
		}

		if recoveredID != id {
			t.Errorf("ID mismatch for format %d: original=%d, recovered=%d, readable=%s",
				format, id, recoveredID, readable)
		}

		// 验证长度
		expectedLen := expectedLength(format)
		if len(readable) != expectedLen {
			t.Errorf("Length mismatch for format %d: expected=%d, actual=%d, readable=%s",
				format, expectedLen, len(readable), readable)
		}
	}
}

func TestToReadableFormatLength(t *testing.T) {
	gen, _ := NewGenerator(123)
	id, _ := gen.Generate()

	tests := []struct {
		format      ReadableFormat
		expectedLen int
	}{
		{FormatYYMMDD, 21},
		{FormatYYYYMMDD, 23},
		{FormatYYMMDDHHMMSS, 22},
		{FormatYYYYMMDDHHMMSS, 24},
	}

	for _, tt := range tests {
		readable := gen.ToReadableWithFormat(id, tt.format)
		if len(readable) != tt.expectedLen {
			t.Errorf("Format %d: expected length %d, got %d, readable=%s",
				tt.format, tt.expectedLen, len(readable), readable)
		}

		// 验证全数字
		for _, c := range readable {
			if c < '0' || c > '9' {
				t.Errorf("Format %d: non-digit character in readable=%s", tt.format, readable)
				break
			}
		}
	}
}

func TestFromReadableInvalid(t *testing.T) {
	gen, _ := NewGenerator(123)

	tests := []struct {
		name     string
		readable string
		format   ReadableFormat
		wantErr  bool
	}{
		{"too short", "12345", FormatYYMMDD, true},
		{"too long", "123456789012345678901234567890", FormatYYMMDD, true},
		{"year out of range YY", "24082412345678901234", FormatYYMMDD, true},
		{"year out of range YY", "95082412345678901234", FormatYYMMDD, true},
		{"invalid month", "25002412345678901234", FormatYYMMDD, true},
		{"invalid day", "25083212345678901234", FormatYYMMDD, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gen.FromReadable(tt.readable, tt.format)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for readable=%s, got nil", tt.readable)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error for readable=%s: %v", tt.readable, err)
			}
		})
	}
}

func TestToReadableMultipleIDs(t *testing.T) {
	gen, _ := NewGenerator(123)

	// 生成多个ID，验证格式一致性
	for i := 0; i < 10; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		readableYY := gen.ToReadableWithFormat(id, FormatYYMMDD)
		readableYYYY := gen.ToReadableWithFormat(id, FormatYYYYMMDD)

		// YYYY格式应包含YY格式的前缀（去掉前两位"20"）
		if readableYYYY[2:8] != readableYY[:6] {
			t.Errorf("Prefix mismatch: YYYY=%s, YY=%s", readableYYYY, readableYY)
		}

		// 验证可逆
		recoveredYY, err := gen.FromReadable(readableYY, FormatYYMMDD)
		if err != nil || recoveredYY != id {
			t.Errorf("YY reversible failed: id=%d, recovered=%d, err=%v", id, recoveredYY, err)
		}

		recoveredYYYY, err := gen.FromReadable(readableYYYY, FormatYYYYMMDD)
		if err != nil || recoveredYYYY != id {
			t.Errorf("YYYY reversible failed: id=%d, recovered=%d, err=%v", id, recoveredYYYY, err)
		}
	}
}

func BenchmarkToReadableWithFormat(b *testing.B) {
	gen, _ := NewGenerator(123)
	id, _ := gen.Generate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.ToReadableWithFormat(id, FormatYYYYMMDD)
	}
}

func BenchmarkFromReadable(b *testing.B) {
	gen, _ := NewGenerator(123)
	id, _ := gen.Generate()
	readable := gen.ToReadableWithFormat(id, FormatYYYYMMDD)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.FromReadable(readable, FormatYYYYMMDD)
	}
}

func TestIncompatibleSettings(t *testing.T) {
	// 使用非默认配置创建生成器
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 6, // 非 9
		TimelineBit:  1,
		SeqBit:       15, // 非 12
		Epoch:        DefaultEpoch,
	}
	gen, err := NewGeneratorWithSettings(0, settings)
	if err != nil {
		t.Fatalf("NewGeneratorWithSettings failed: %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// ToReadable 应该 panic
	t.Run("ToReadable panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("ToReadableWithFormat did not panic for incompatible settings")
			}
		}()
		gen.ToReadableWithFormat(id, FormatYYYYMMDD)
	})

	// FromReadable 应该返回错误
	t.Run("FromReadable error", func(t *testing.T) {
		// 使用一个有效的可读格式字符串
		_, err := gen.FromReadable("202504171200001234567890", FormatYYYYMMDD)
		if err == nil {
			t.Errorf("FromReadable did not return error for incompatible settings")
		}
	})
}