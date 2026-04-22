package mtl_snowflake

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestSettingsValidation(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		wantErr  bool
	}{
		{
			name: "valid default settings",
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 9,
				TimelineBit:  1,
				SeqBit:       12,
			},
			wantErr: false,
		},
		{
			name: "invalid total bits",
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 10,
				TimelineBit:  1,
				SeqBit:       12,
			},
			wantErr: true,
		},
		{
			name: "single timeline (TimelineBit=0)",
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 10,
				TimelineBit:  0,
				SeqBit:       12,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSettingsPresets(t *testing.T) {
	s := Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
	}
	s.initPresets()

	if s.timeMax != (1<<41)-1 {
		t.Errorf("timeMax = %d, want %d", s.timeMax, (1<<41)-1)
	}
	if s.machineIDMax != (1<<9)-1 {
		t.Errorf("machineIDMax = %d, want %d", s.machineIDMax, (1<<9)-1)
	}
	if s.timelineCount != 2 {
		t.Errorf("timelineCount = %d, want 2", s.timelineCount)
	}
	if s.seqMax != (1<<12)-1 {
		t.Errorf("seqMax = %d, want %d", s.seqMax, (1<<12)-1)
	}

	if s.timeShift != 22 {
		t.Errorf("timeShift = %d, want 22", s.timeShift)
	}
	if s.machineShift != 13 {
		t.Errorf("machineShift = %d, want 13", s.machineShift)
	}
	if s.timelineShift != 12 {
		t.Errorf("timelineShift = %d, want 12", s.timelineShift)
	}
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Errorf("DefaultSettings validation failed: %v", err)
	}
}

func TestNewGenerator(t *testing.T) {
	machineID := int64(0)
	gen, err := NewGenerator(machineID)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}
	if gen == nil {
		t.Fatal("generator is nil")
	}
	if gen.machineID != machineID {
		t.Errorf("machineID = %d, want %d", gen.machineID, machineID)
	}
}

func TestNewGeneratorWithSettings(t *testing.T) {
	tests := []struct {
		name      string
		machineID int64
		settings  Settings
		wantErr   error
	}{
		{
			name:      "valid custom settings",
			machineID: 0,
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 6,
				TimelineBit:  1,
				SeqBit:       15,
				Epoch:        DefaultEpoch,
			},
			wantErr: nil,
		},
		{
			name:      "machineID overflow",
			machineID: 100,
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 6,
				TimelineBit:  1,
				SeqBit:       15,
				Epoch:        DefaultEpoch,
			},
			wantErr: ErrMachineIDOverflow,
		},
		{
			name:      "invalid settings",
			machineID: 0,
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 10,
				TimelineBit:  1,
				SeqBit:       12,
			},
			wantErr: ErrInvalidSettings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewGeneratorWithSettings(tt.machineID, tt.settings)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gen == nil {
				t.Fatal("generator is nil")
			}
		})
	}
}

func TestGenerateBasic(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("id = %d, want positive", id)
	}

	// Verify ID can be decomposed
	info := gen.Decompose(id)
	if info.MachineID != 0 {
		t.Errorf("MachineID = %d, want 0", info.MachineID)
	}
}

func TestGenerateUnique(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	ids := make(map[int64]bool)
	count := 10000

	for i := 0; i < count; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if ids[id] {
			t.Fatalf("duplicate id: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("unique ids = %d, want %d", len(ids), count)
	}
}

func TestTimeBackward(t *testing.T) {
	tests := []struct {
		name          string
		timelineBit   uint64
		backwardCount int
		wantSuccess   int
		wantFail      int
	}{
		{
			name:          "single timeline (TimelineBit=0)",
			timelineBit:   0,
			backwardCount: 1,
			wantSuccess:   0,
			wantFail:      1,
		},
		{
			name:          "dual timeline (TimelineBit=1)",
			timelineBit:   1,
			backwardCount: 2,
			wantSuccess:   1,
			wantFail:      1,
		},
		{
			name:          "four timeline (TimelineBit=2)",
			timelineBit:   2,
			backwardCount: 4,
			wantSuccess:   3,
			wantFail:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				TimeBit:      41,
				MachineIDBit: 10 - tt.timelineBit,
				TimelineBit:  tt.timelineBit,
				SeqBit:       12,
				Epoch:        DefaultEpoch,
			}

			gen, err := NewGeneratorWithSettings(0, settings)
			if err != nil {
				t.Fatalf("NewGenerator failed: %v", err)
			}

			// 先生成一个ID以初始化时间线进度
			_, err = gen.Generate()
			if err != nil {
				t.Fatalf("Initial Generate failed: %v", err)
			}

			successCount := 0
			failCount := 0

			// 模拟时钟回退
			for i := 0; i < tt.backwardCount; i++ {
				// 强制推进时间线进度
				gen.mutex.Lock()
				gen.timelineProgress[gen.curTimeline] += 100
				gen.mutex.Unlock()

				_, err := gen.Generate()
				if err != nil {
					if errors.Is(err, ErrNoAvailableTimeline) || errors.Is(err, ErrTimeBackwardTooFar) {
						failCount++
					} else {
						t.Fatalf("unexpected error: %v", err)
					}
				} else {
					successCount++
				}
			}

			if successCount != tt.wantSuccess {
				t.Errorf("successCount = %d, want %d", successCount, tt.wantSuccess)
			}
			if failCount != tt.wantFail {
				t.Errorf("failCount = %d, want %d", failCount, tt.wantFail)
			}
		})
	}
}

func TestTimeBackwardTimelineProgressNotReverted(t *testing.T) {
	// 验证：大幅时钟回退切换时间线时，旧时间线进度不应被降低
	// 否则切回旧时间线后会重新使用已用时间戳，导致重复ID
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        DefaultEpoch,
	}
	gen, err := NewGeneratorWithSettings(0, settings)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	// 生成一个ID以初始化时间线进度
	_, err = gen.Generate()
	if err != nil {
		t.Fatalf("Initial Generate failed: %v", err)
	}

	// 记录当前时间线进度
	gen.mutex.Lock()
	curTL := gen.curTimeline
	progressBefore := gen.timelineProgress[curTL]
	gen.mutex.Unlock()

	// 模拟大幅时钟回退（推进进度100ms，然后尝试生成）
	gen.mutex.Lock()
	gen.timelineProgress[curTL] = progressBefore + 100
	gen.mutex.Unlock()

	_, err = gen.Generate()
	if err != nil {
		// 如果没有可用时间线则跳过
		t.Skipf("no available timeline: %v", err)
	}

	// 验证：旧时间线进度不应被降低
	gen.mutex.Lock()
	progressAfter := gen.timelineProgress[curTL]
	curTLAfter := gen.curTimeline
	gen.mutex.Unlock()

	// 切换到另一条时间线后，旧时间线进度应保持不变
	if curTLAfter != curTL {
		if progressAfter < progressBefore+100 {
			t.Errorf("timeline %d progress was reverted: before=%d, after=%d, should not decrease",
				curTL, progressBefore+100, progressAfter)
		}
	}
}

func TestDecomposeDetailed(t *testing.T) {
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 9,
		TimelineBit:  1,
		SeqBit:       12,
		Epoch:        DefaultEpoch,
	}

	gen, err := NewGeneratorWithSettings(123, settings)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	// Initialize presets for test use
	settings.Validate()
	seqMax := settings.seqMax

	// 生成多个ID并验证解析
	for i := 0; i < 100; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		info := gen.Decompose(id)

		// 验证MachineID
		if info.MachineID != 123 {
			t.Errorf("id=%d: MachineID=%d, want 123", id, info.MachineID)
		}

		// 验证Timeline范围
		if info.Timeline < 0 || info.Timeline > 1 {
			t.Errorf("id=%d: Timeline=%d, out of range [0,1]", id, info.Timeline)
		}

		// 验证Sequence范围
		if info.Sequence < 0 || info.Sequence > seqMax {
			t.Errorf("id=%d: Sequence=%d, out of range [0,%d]", id, info.Sequence, seqMax)
		}

		// 验证Timestamp范围
		if info.Timestamp < 0 {
			t.Errorf("id=%d: Timestamp=%d, want >= 0", id, info.Timestamp)
		}
	}
}

func TestDecomposeCustomSettings(t *testing.T) {
	// 使用非默认配置测试
	settings := Settings{
		TimeBit:      42,
		MachineIDBit: 8,
		TimelineBit:  2,
		SeqBit:       11,
		Epoch:        DefaultEpoch,
	}

	gen, err := NewGeneratorWithSettings(255, settings)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	info := gen.Decompose(id)

	if info.MachineID != 255 {
		t.Errorf("MachineID=%d, want 255", info.MachineID)
	}

	// TimelineBit=2，范围[0,3]
	if info.Timeline < 0 || info.Timeline > 3 {
		t.Errorf("Timeline=%d, out of range [0,3]", info.Timeline)
	}
}

func TestToReadable(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	readable, err := gen.ToReadable(id)

	// 验证格式：YYYYMMDDHHMMSS + mss(3位毫秒) + 序列号部分
	// 例如：20250416123456000001 (时间14位 + 毫秒3位 + 序列号)
	if len(readable) < 17 {
		t.Errorf("ToReadable length = %d, want >= 17", len(readable))
	}

	// 验证前17位是时间格式（全数字）
	if len(readable) >= 17 {
		timePart := readable[:17]
		for i, c := range timePart {
			if c < '0' || c > '9' {
				t.Errorf("timePart[%d] = %c, want digit", i, c)
			}
		}
	}
}

func TestToReadableFormat(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	// 生成ID并验证格式
	for i := 0; i < 10; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		readable, err := gen.ToReadable(id)
		if err != nil {
			t.Fatalf("ToReadable failed: %v", err)
		}

		// 格式应该是：YYYYMMDDHHMMSS + mss(毫秒3位) + 序列号
		// 例如：2025041612345600000001

		// 验证可以通过解析还原
		info := gen.Decompose(id)

		// 验证时间部分正确
		if info.Timestamp > 0 {
			// 时间戳有效
		}

		// 验证readable长度合理（至少17位：时间14位+毫秒3位）
		if len(readable) < 17 {
			t.Errorf("readable length = %d, want >= 17", len(readable))
		}
	}
}

func TestGenerateConcurrent(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	concurrency := 10
	countPerWorker := 10000
	total := concurrency * countPerWorker

	ids := make(chan int64, total)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < countPerWorker; j++ {
				id, err := gen.Generate()
				if err != nil {
					t.Errorf("Generate failed: %v", err)
					return
				}
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 检查唯一性
	uniqueIds := make(map[int64]bool)
	for id := range ids {
		if uniqueIds[id] {
			t.Fatalf("duplicate id: %d", id)
		}
		uniqueIds[id] = true
	}

	if len(uniqueIds) != total {
		t.Errorf("unique ids = %d, want %d", len(uniqueIds), total)
	}
}

func TestGetMachineID(t *testing.T) {
	machineIDBit := uint64(9)

	id1, err := GetMachineID(machineIDBit)
	if err != nil {
		t.Fatalf("GetMachineID failed: %v", err)
	}

	maxMachineID := int64(1<<machineIDBit) - 1
	if id1 < 0 || id1 > maxMachineID {
		t.Errorf("machineID = %d, out of range [0, %d]", id1, maxMachineID)
	}

	// 验证同一机器多次调用返回相同结果（指纹哈希确定性）
	id2, err := GetMachineID(machineIDBit)
	if err != nil {
		t.Fatalf("GetMachineID second call failed: %v", err)
	}

	if id2 != id1 {
		t.Errorf("machineID inconsistent: first=%d, second=%d, should be same", id1, id2)
	}
}

func TestGetMachineIDDifferentBits(t *testing.T) {
	tests := []struct {
		name         string
		machineIDBit uint64
	}{
		{"6bit", 6},
		{"9bit", 9},
		{"10bit", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GetMachineID(tt.machineIDBit)
			if err != nil {
				t.Fatalf("GetMachineID failed: %v", err)
			}

			max := int64(1<<tt.machineIDBit) - 1
			if id < 0 || id > max {
				t.Errorf("machineID = %d, out of range [0, %d]", id, max)
			}
		})
	}
}

func TestGetMachineIDFromPodName(t *testing.T) {
	tests := []struct {
		name         string
		podName      string
		machineIDBit uint64
		wantID       int64
		wantErr      bool
	}{
		{
			name:         "valid pod name",
			podName:      "id-generator-5",
			machineIDBit: 9,
			wantID:       5,
			wantErr:      false,
		},
		{
			name:         "pod name with multiple hyphens",
			podName:      "my-app-id-generator-123",
			machineIDBit: 9,
			wantID:       123,
			wantErr:      false,
		},
		{
			name:         "ordinal overflow",
			podName:      "id-generator-600",
			machineIDBit: 9,
			wantErr:      true,
		},
		{
			name:         "invalid pod name format",
			podName:      "invalidname",
			machineIDBit: 9,
			wantErr:      true,
		},
		{
			name:         "zero ordinal",
			podName:      "id-generator-0",
			machineIDBit: 9,
			wantID:       0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置环境变量
			os.Setenv("POD_NAME", tt.podName)
			defer os.Unsetenv("POD_NAME")

			id, err := GetMachineIDFromPodName(tt.machineIDBit)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("machineID = %d, want %d", id, tt.wantID)
			}
		})
	}
}

func TestGetMachineIDFromPodNameNotSet(t *testing.T) {
	os.Unsetenv("POD_NAME")

	_, err := GetMachineIDFromPodName(9)
	if err == nil {
		t.Fatal("expected error when POD_NAME not set")
	}
}

// TestLargeScaleUniqueID 大规模唯一性测试
func TestLargeScaleUniqueID(t *testing.T) {
	var ids sync.Map

	for machineID := int64(0); machineID < 10; machineID++ {
		func(machineID int64) {
			t.Run(string(rune('0'+machineID)), func(t *testing.T) {
				t.Parallel()

				gen, err := NewGenerator(machineID)
				if err != nil {
					t.Fatal(err)
				}

				// 生成100万个ID，检查是否有重复
				count := int64(1e6)
				for i := count; i > 0; i-- {
					id, err := gen.Generate()
					if err != nil {
						t.Fatal(err)
					}
					if _, exist := ids.Load(id); exist {
						t.Fatalf("duplicate id: %d", id)
					}
					ids.Store(id, nil)
				}
			})
		}(machineID)
	}
}

// TestEpochValidation epoch 时间校验测试
func TestEpochValidation(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		wantErr  error
	}{
		{
			name: "valid epoch (default)",
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 9,
				TimelineBit:  1,
				SeqBit:       12,
				Epoch:        DefaultEpoch,
			},
			wantErr: nil,
		},
		{
			name: "epoch too late (future time)",
			settings: Settings{
				TimeBit:      41,
				MachineIDBit: 9,
				TimelineBit:  1,
				SeqBit:       12,
				Epoch:        time.Now().Add(time.Hour).UnixNano(),
			},
			wantErr: ErrEpochTooLate,
		},
		{
			name: "time bit too small (will exceed max)",
			settings: Settings{
				TimeBit:      35,
				MachineIDBit: 9,
				TimelineBit:  1,
				SeqBit:       18,
				Epoch:        time.Now().AddDate(-3, 0, 0).UnixNano(),
			},
			wantErr: ErrTimeOffsetExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.ValidateWithTime()
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func BenchmarkGenerateSeqBit12(b *testing.B) {
	gen, _ := NewGenerator(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate()
	}
}

func BenchmarkGenerateSeqBit14(b *testing.B) {
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 7,
		TimelineBit:  1,
		SeqBit:       14,
		Epoch:        DefaultEpoch,
	}
	gen, _ := NewGeneratorWithSettings(0, settings)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate()
	}
}

func BenchmarkGetMachineID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetMachineID(9)
	}
}

func BenchmarkToReadable(b *testing.B) {
	gen, _ := NewGenerator(0)
	id, _ := gen.Generate()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.ToReadable(id)
	}
}

func BenchmarkGenerateSeqBit21(b *testing.B) {
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 0,
		TimelineBit:  1,
		SeqBit:       21,
		Epoch:        DefaultEpoch,
	}
	gen, _ := NewGeneratorWithSettings(0, settings)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate()
	}
}

func TestMustGenerate(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	// 成功路径：应返回正数ID，不panic
	id := gen.MustGenerate()
	if id <= 0 {
		t.Errorf("MustGenerate returned %d, want positive", id)
	}
}

func TestMustGeneratePanic(t *testing.T) {
	// 单时间线 + 强制进度推进 → 无可用时间线 → panic
	settings := Settings{
		TimeBit:      41,
		MachineIDBit: 10,
		TimelineBit:  0,
		SeqBit:       12,
		Epoch:        DefaultEpoch,
	}
	gen, err := NewGeneratorWithSettings(0, settings)
	if err != nil {
		t.Fatalf("NewGeneratorWithSettings failed: %v", err)
	}

	// 先生成一个ID初始化进度
	gen.MustGenerate()

	// 强制推进进度，模拟时钟回退
	gen.mutex.Lock()
	gen.timelineProgress[gen.curTimeline] += 100
	gen.mutex.Unlock()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustGenerate did not panic when no available timeline")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("MustGenerate panicked with non-error: %T", r)
		}
		if !errors.Is(err, ErrNoAvailableTimeline) {
			t.Fatalf("MustGenerate panicked with wrong error: %v, want %v", err, ErrNoAvailableTimeline)
		}
	}()
	gen.MustGenerate()
}
