package proxy

import (
	"bytes"
	go2pool "go2/pool"
	"io"
	"testing"
)

func TestMain(m *testing.M) {
	// copyBuf / writeFrame 都从 go2/pool 取缓冲, 与 main.go 保持一致先初始化
	go2pool.Init()
	m.Run()
}

// 隧道是字节流, 分帧必须能把多个数据报原样还原出来
func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	datagrams := [][]byte{
		[]byte("hello"),
		[]byte(""),
		[]byte("a-much-longer-datagram-payload"),
		{0x00, 0xff, 0x10, 0x7f},
		bytes.Repeat([]byte{0x5a}, 1200),
	}

	for _, d := range datagrams {
		if err := writeFrame(&buf, d); err != nil {
			t.Fatalf("writeFrame 失败: %v", err)
		}
	}

	out := make([]byte, copyBufSize)
	for i, want := range datagrams {
		n, err := readFrame(&buf, out)
		if err != nil {
			t.Fatalf("第%d个数据报 readFrame 失败: %v", i, err)
		}
		if n != len(want) || !bytes.Equal(out[:n], want) {
			t.Fatalf("第%d个数据报边界错乱: got %d/%x want %d/%x", i, n, out[:n], len(want), want)
		}
	}

	// 所有数据报读完后缓冲区应为空
	if buf.Len() != 0 {
		t.Fatalf("仍有未读取的字节: %d", buf.Len())
	}
}

// 数据报被拆到多次写里时, 读侧仍要按帧重组出完整数据报
func TestReadFrameAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("split-payload")
	if err := writeFrame(&buf, payload); err != nil {
		t.Fatalf("writeFrame 失败: %v", err)
	}
	// 只写入帧头和第一个字节, 模拟流被拆开
	raw := buf.Bytes()
	var partial bytes.Buffer
	partial.Write(raw[:3])

	out := make([]byte, copyBufSize)
	done := make(chan error, 1)
	nCh := make(chan int, 1)
	go func() {
		n, err := readFrame(&partial, out)
		nCh <- n
		done <- err
	}()

	partial.Write(raw[3:])
	if err := <-done; err != nil {
		t.Fatalf("readFrame 失败: %v", err)
	}
	if n := <-nCh; n != len(payload) || !bytes.Equal(out[:n], payload) {
		t.Fatalf("重组结果错误: %d/%x", n, out[:n])
	}
}

func TestWriteFrameRejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, make([]byte, maxUDPFramePayload+1)); err == nil {
		t.Fatal("超过上限的数据报应当被拒绝")
	}
	if buf.Len() != 0 {
		t.Fatalf("被拒绝时不该写入任何字节, 实际写入 %d", buf.Len())
	}
}

// 帧长度非法时必须报错, 否则流的帧边界会一直错位
func TestReadFrameRejectsBadLength(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xff, 0xff}) // 65535, 超过提供的缓冲区
	out := make([]byte, 64)
	if _, err := readFrame(&buf, out); err == nil {
		t.Fatal("非法帧长度应当返回错误")
	}

	// 零长度数据报是合法的, 不能被当成错误帧, 否则整条流会被关闭
	buf.Reset()
	buf.Write([]byte{0x00, 0x00})
	if n, err := readFrame(&buf, out); err != nil || n != 0 {
		t.Fatalf("零长度数据报应当被正常读出: n=%d err=%v", n, err)
	}
}

// 尾部不完整的帧不能返回成功
func TestReadFrameRejectsTruncated(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x10}) // 声明 16 字节, 实际只给 3 字节
	buf.Write([]byte{1, 2, 3})

	out := make([]byte, copyBufSize)
	if _, err := readFrame(&buf, out); err != io.ErrUnexpectedEOF && err == nil {
		t.Fatal("截断的帧应当返回错误")
	}
}
