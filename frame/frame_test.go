package frame

import (
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestHeaderConfig_Parse 测试头部解析功能
func TestHeaderConfig_Parse(t *testing.T) {
	tests := []struct {
		name           string
		config         *HeaderConfig
		header         []byte
		expectedLength int
		expectedError  bool
		errorMessage   string
	}{
		// 正常情况测试
		{
			name: "正常解析2字节长度字段-大端序",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			header:         []byte{0x00, 0x10}, // 16
			expectedLength: 16,
			expectedError:  false,
		},
		{
			name: "正常解析2字节长度字段-小端序",
			config: &HeaderConfig{
				ByteOrder:         binary.LittleEndian,
				LengthFieldLength: 2,
			},
			header:         []byte{0x10, 0x00}, // 16
			expectedLength: 16,
			expectedError:  false,
		},
		{
			name: "正常解析4字节长度字段-大端序",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 4,
			},
			header:         []byte{0x00, 0x00, 0x01, 0x00}, // 256
			expectedLength: 256,
			expectedError:  false,
		},
		{
			name: "正常解析4字节长度字段-小端序",
			config: &HeaderConfig{
				ByteOrder:         binary.LittleEndian,
				LengthFieldLength: 4,
			},
			header:         []byte{0x00, 0x01, 0x00, 0x00}, // 256
			expectedLength: 256,
			expectedError:  false,
		},
		// 边界条件测试
		{
			name: "最小长度-2字节字段值为0",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			header:         []byte{0x00, 0x00}, // 0
			expectedLength: 0,
			expectedError:  false,
		},
		{
			name: "最大长度-2字节字段",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			header:         []byte{0xFF, 0xFF}, // 65535
			expectedLength: 65535,
			expectedError:  false,
		},
		{
			name: "最大长度-4字节字段",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 4,
			},
			header:         []byte{0x7F, 0xFF, 0xFF, 0xFF}, // 2147483647
			expectedLength: 2147483647,
			expectedError:  false,
		},
		// 异常情况测试
		{
			name: "头部数据不足-2字节字段只有1字节",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			header:        []byte{0x00},
			expectedError: true,
			errorMessage:  "header too short",
		},
		{
			name: "头部数据不足-4字节字段只有3字节",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 4,
			},
			header:        []byte{0x00, 0x00, 0x01},
			expectedError: true,
			errorMessage:  "header too short",
		},
		{
			name: "空头部数据",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			header:        []byte{},
			expectedError: true,
			errorMessage:  "header too short",
		},
		{
			name: "不支持的长度字段长度",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 3,
			},
			header:        []byte{0x00, 0x00, 0x01},
			expectedError: true,
			errorMessage:  "unsupported LengthFieldLength, only 2 or 4",
		},
		{
			name: "不支持的长度字段长度-1字节",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 1,
			},
			header:        []byte{0x10},
			expectedError: true,
			errorMessage:  "unsupported LengthFieldLength, only 2 or 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length, err := tt.config.Parse(tt.header)

			if tt.expectedError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				} else if err.Error() != tt.errorMessage {
					t.Errorf("错误信息不匹配，期望: %s, 实际: %s", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误，但出现了错误: %v", err)
				}
				if length != tt.expectedLength {
					t.Errorf("长度不匹配，期望: %d, 实际: %d", tt.expectedLength, length)
				}
			}
		})
	}
}

// TestFrame_ReadFrame 测试数据包读取功能
func TestFrame_ReadFrame(t *testing.T) {
	tests := []struct {
		name           string
		config         *HeaderConfig
		inputData      [][]byte // 模拟多次输入
		expectedFrames [][]byte // 期望输出的完整包
		expectedError  bool
		errorMessage   string
	}{
		// 正常情况测试 - 无分包
		{
			name: "单个完整包-2字节头部",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}, // 长度5 + "hello"
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
			},
			expectedError: false,
		},
		{
			name: "单个完整包-4字节头部",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 4,
			},
			inputData: [][]byte{
				{0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}, // 长度5 + "hello"
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
			},
			expectedError: false,
		},
		// 分包情况测试
		{
			name: "头部分包-分两次接收",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00},                          // 头部第一字节
				{0x05, 'h', 'e', 'l', 'l', 'o'}, // 头部第二字节 + 完整body
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
			},
			expectedError: false,
		},
		{
			name: "数据体分包-分多次接收",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00, 0x05, 'h', 'e'}, // 头部 + 部分body
				{'l', 'l'},             // 继续body
				{'o'},                  // 完成body
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
			},
			expectedError: false,
		},
		{
			name: "多个包连续接收",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00, 0x05, 'h', 'e', 'l', 'l', 'o', 0x00, 0x05, 'w', 'o', 'r', 'l', 'd'},
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
				{'w', 'o', 'r', 'l', 'd'},
			},
			expectedError: false,
		},
		{
			name: "多个包分批接收",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00, 0x05, 'h', 'e', 'l', 'l', 'o', 0x00, 0x05, 'w', 'o'},
				{'r', 'l', 'd'},
			},
			expectedFrames: [][]byte{
				{'h', 'e', 'l', 'l', 'o'},
				{'w', 'o', 'r', 'l', 'd'},
			},
			expectedError: false,
		},
		// 边界条件测试
		{
			name: "空数据包",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				{0x00, 0x00}, // 长度为0
			},
			expectedFrames: [][]byte{
				{}, // 空包体
			},
			expectedError: false,
		},
		{
			name: "大数据包",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				append([]byte{0x04, 0x00}, make([]byte, 1024)...), // 1024字节数据包
			},
			expectedFrames: [][]byte{
				make([]byte, 1024),
			},
			expectedError: false,
		},
		{
			name: "大数据包分包",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			inputData: [][]byte{
				append([]byte{0x04, 0x00}, make([]byte, 24)...), // 1024字节数据包
				make([]byte, 1000),
			},
			expectedFrames: [][]byte{

				make([]byte, 1024),
			},
			expectedError: false,
		},
		// 异常情况测试
		{
			name: "头部解析错误",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 3, // 不支持的长度
			},
			inputData: [][]byte{
				{0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'},
			},
			expectedFrames: nil,
			expectedError:  true,
			errorMessage:   "unsupported LengthFieldLength, only 2 or 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &Frame{
				Hc:  tt.config,
				buf: make([]byte, 0),
			}

			var actualFrames [][]byte
			var err error

			// 模拟多次数据输入
			for _, input := range tt.inputData {
				for {
					var frameData []byte
					frameData, err = frame.ReadFrame(input)

					if err != nil {
						break
					}

					if frameData != nil {
						actualFrames = append(actualFrames, frameData)
						input = []byte{} // 后续循环不再输入新数据
					} else {
						break // 数据不足，等待下次输入
					}
				}

				if err != nil {
					break
				}
			}

			if tt.expectedError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				} else if err.Error() != tt.errorMessage {
					t.Errorf("错误信息不匹配，期望: %s, 实际: %s", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误，但出现了错误: %v", err)
				}

				if len(actualFrames) != len(tt.expectedFrames) {
					t.Errorf("包数量不匹配，期望: %d, 实际: %d", len(tt.expectedFrames), len(actualFrames))
				}

				for i, expectedFrame := range tt.expectedFrames {
					if i >= len(actualFrames) {
						t.Errorf("缺少第 %d 个包", i+1)
						continue
					}

					if !bytesEqual(actualFrames[i], expectedFrame) {
						t.Errorf("第 %d 个包内容不匹配，期望: %v, 实际: %v", i+1, expectedFrame, actualFrames[i])
					}
				}
			}
		})
	}
}

// TestFrame_ReadFrame_Concurrent 并发测试
func TestFrame_ReadFrame_Concurrent(t *testing.T) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}

	frame := &Frame{
		Hc:  config,
		buf: make([]byte, 0),
	}

	const numGoroutines = 10
	const numPacketsPerGoroutine = 100

	var wg sync.WaitGroup
	results := make([][]byte, numGoroutines*numPacketsPerGoroutine)
	errors := make([]error, numGoroutines*numPacketsPerGoroutine)

	// 并发写入数据包
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numPacketsPerGoroutine; j++ {
				data := []byte{0x00, 0x04} // 长度4
				data = append(data, byte(goroutineID), byte(j), byte(goroutineID), byte(j))

				result, err := frame.ReadFrame(data)
				idx := goroutineID*numPacketsPerGoroutine + j
				results[idx] = result
				errors[idx] = err
			}
		}(i)
	}

	wg.Wait()

	// 验证结果
	successCount := 0
	for i, err := range errors {
		if err != nil {
			t.Errorf("第 %d 次调用出现错误: %v", i, err)
		} else if results[i] != nil {
			successCount++
		}
	}

	if successCount == 0 {
		t.Error("并发测试中没有成功解析任何包")
	}

	t.Logf("并发测试完成，成功解析 %d 个包", successCount)
}

// TestFrame_ReadFrame_Performance 性能测试
func TestFrame_ReadFrame_Performance(t *testing.T) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}

	frame := &Frame{
		Hc:  config,
		buf: make([]byte, 0),
	}

	// 准备测试数据
	testData := make([]byte, 1000) // 1KB数据包
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	packet := append([]byte{0x03, 0xE8}, testData...) // 长度1000 + 数据

	const iterations = 10000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, err := frame.ReadFrame(packet)
		if err != nil {
			t.Fatalf("性能测试中出现错误: %v", err)
		}
	}

	duration := time.Since(start)
	packetsPerSecond := float64(iterations) / duration.Seconds()

	t.Logf("性能测试结果: 处理 %d 个包耗时 %v, 平均每秒处理 %.2f 个包",
		iterations, duration, packetsPerSecond)

	// 性能基准：至少每秒处理1000个包
	if packetsPerSecond < 1000 {
		t.Errorf("性能不达标，每秒处理包数: %.2f, 期望至少: 1000", packetsPerSecond)
	}
}

// TestFrame_ReadFrame_MemoryUsage 内存使用测试
func TestFrame_ReadFrame_MemoryUsage(t *testing.T) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}

	frame := &Frame{
		Hc:  config,
		buf: make([]byte, 0),
	}

	// 测试缓冲区是否正确清理
	largeData := make([]byte, 10000)
	packet := append([]byte{0x27, 0x10}, largeData...) // 长度10000

	_, err := frame.ReadFrame(packet)
	if err != nil {
		t.Fatalf("读取大包时出现错误: %v", err)
	}

	// 验证缓冲区已清空
	if len(frame.buf) != 0 {
		t.Errorf("缓冲区未正确清理，剩余长度: %d", len(frame.buf))
	}
}

// TestFrame_ReadFrame_EdgeCases 边界情况测试
func TestFrame_ReadFrame_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		config   *HeaderConfig
		scenario func(*testing.T, *Frame)
	}{
		{
			name: "连续调用空数据",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			scenario: func(t *testing.T, frame *Frame) {
				for i := 0; i < 5; i++ {
					result, err := frame.ReadFrame([]byte{})
					if err != nil {
						t.Errorf("第 %d 次调用出现错误: %v", i+1, err)
					}
					if result != nil {
						t.Errorf("第 %d 次调用应该返回nil，但返回了: %v", i+1, result)
					}
				}
			},
		},
		{
			name: "逐字节输入完整包",
			config: &HeaderConfig{
				ByteOrder:         binary.BigEndian,
				LengthFieldLength: 2,
			},
			scenario: func(t *testing.T, frame *Frame) {
				fullPacket := []byte{0x00, 0x03, 'a', 'b', 'c'}
				var result []byte
				var err error

				for i, b := range fullPacket {
					result, err = frame.ReadFrame([]byte{b})
					if err != nil {
						t.Errorf("第 %d 字节输入时出现错误: %v", i+1, err)
						return
					}

					if i < len(fullPacket)-1 {
						if result != nil {
							t.Errorf("第 %d 字节输入时不应该返回完整包", i+1)
						}
					}
				}

				if result == nil {
					t.Error("最后应该返回完整包")
				} else if !bytesEqual(result, []byte{'a', 'b', 'c'}) {
					t.Errorf("包内容不正确，期望: %v, 实际: %v", []byte{'a', 'b', 'c'}, result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &Frame{
				Hc:  tt.config,
				buf: make([]byte, 0),
			}
			tt.scenario(t, frame)
		})
	}
}

// TestFrame_ReadFrame_DataIntegrity 数据完整性测试
func TestFrame_ReadFrame_DataIntegrity(t *testing.T) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}

	frame := &Frame{
		Hc:  config,
		buf: make([]byte, 0),
	}

	// 测试各种数据模式
	testPatterns := [][]byte{
		{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}, // 递增
		{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA}, // 递减
		{0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA}, // 重复
		{0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF}, // 交替
	}

	for i, pattern := range testPatterns {
		t.Run(fmt.Sprintf("数据模式_%d", i+1), func(t *testing.T) {
			packet := make([]byte, 2+len(pattern))
			binary.BigEndian.PutUint16(packet[:2], uint16(len(pattern)))
			copy(packet[2:], pattern)

			result, err := frame.ReadFrame(packet)
			if err != nil {
				t.Errorf("解析数据模式 %d 时出现错误: %v", i+1, err)
			}

			if !bytesEqual(result, pattern) {
				t.Errorf("数据模式 %d 完整性验证失败，期望: %v, 实际: %v", i+1, pattern, result)
			}
		})
	}
}

// 辅助函数：比较字节切片
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 基准测试
func BenchmarkHeaderConfig_Parse2Bytes(b *testing.B) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}
	header := []byte{0x01, 0x00}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = config.Parse(header)
	}
}

func BenchmarkHeaderConfig_Parse4Bytes(b *testing.B) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 4,
	}
	header := []byte{0x00, 0x00, 0x01, 0x00}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = config.Parse(header)
	}
}

func BenchmarkFrame_ReadFrame(b *testing.B) {
	config := &HeaderConfig{
		ByteOrder:         binary.BigEndian,
		LengthFieldLength: 2,
	}

	frame := &Frame{
		Hc:  config,
		buf: make([]byte, 0),
	}

	packet := []byte{0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = frame.ReadFrame(packet)
	}
}
