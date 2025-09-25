package frame

import (
	"encoding/binary"
	"errors"
	"sync"
)

type Frame struct {
	Hc   *HeaderConfig
	buf  []byte
	lock sync.Mutex
}

type HeaderConfig struct {
	ByteOrder         binary.ByteOrder
	LengthFieldLength int // 长度字段占用字节数（2 或 4）
}

// Parse 根据配置解析出包体总长度（body 的长度，不包含长度字段本身）
func (hc *HeaderConfig) Parse(header []byte) (int, error) {
	if len(header) < hc.LengthFieldLength {
		return 0, errors.New("header too short")
	}

	switch hc.LengthFieldLength {
	case 2:
		return int(hc.ByteOrder.Uint16(header)), nil
	case 4:
		return int(hc.ByteOrder.Uint32(header)), nil
	default:
		return 0, errors.New("unsupported LengthFieldLength, only 2 or 4")
	}
}

// ReadFrame 输入一次从 conn 读到的数据，输出一个完整包（仅 body 部分）
// - 如果数据不足，返回 (nil, nil)，等待下次补充
// - 如果有多个包，调用方需要多次调用 ReadFrame 才能依次取出
func (f *Frame) ReadFrame(raw []byte) ([]byte, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	// 把本次数据追加到缓冲区
	f.buf = append(f.buf, raw...)

	// 先判断是否有足够的 header
	if len(f.buf) < f.Hc.LengthFieldLength {
		return nil, nil
	}

	// 读取包体长度
	bodyLen, err := f.Hc.Parse(f.buf[:f.Hc.LengthFieldLength])
	if err != nil {
		return nil, err
	}

	//f.buf = f.buf[f.Hc.LengthFieldLength:]

	// 总包长度 = header + body
	totalLen := f.Hc.LengthFieldLength + bodyLen

	// 判断数据是否足够
	if len(f.buf) < totalLen {
		return nil, nil // 数据不够，等待下次
	}

	// 拿出一个完整包
	body := f.buf[f.Hc.LengthFieldLength:totalLen]

	// 更新缓冲区，丢掉已消费的部分
	f.buf = f.buf[totalLen:]

	return body, nil
}
