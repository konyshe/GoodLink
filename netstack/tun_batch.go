//go:build windows || darwin

package netstack

// ReadBatch/WriteBatch 直接走 Wintun/utun 的批量接口，避免一包一次系统调用。
func (t *TUN) ReadBatch(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.nt.Read(bufs, sizes, offset)
}

func (t *TUN) WriteBatch(bufs [][]byte, offset int) (int, error) {
	return t.nt.Write(bufs, offset)
}
