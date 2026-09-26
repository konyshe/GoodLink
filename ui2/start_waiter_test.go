package ui2

import (
	"os/exec"
	"sync"
	"testing"
	"time"
)

// 覆盖“停止后立即再启动”时旧 waiter 不能 Wait 新进程、也不能误触发自动重启。
func TestWaiterGenerationIsolatesStopRestart(t *testing.T) {
	id := "waiter-test-" + t.Name()
	defer func() {
		procMu.Lock()
		delete(procs, id)
		procMu.Unlock()
	}()

	sleepCmd := func(sec string) *exec.Cmd {
		return exec.Command("sleep", sec)
	}

	cmd1 := sleepCmd("30")
	if err := cmd1.Start(); err != nil {
		t.Fatalf("start cmd1: %v", err)
	}
	procMu.Lock()
	p := procOf(id)
	p.gen = 1
	p.cmd = cmd1
	p.started = true
	p.autoRestart = true
	procMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		waitForProcessAndHandleExit(id, cmd1, 1)
	}()

	// 尽快停掉第一代, 再立即起第二代
	time.Sleep(20 * time.Millisecond)
	stopServiceProcess(id)

	cmd2 := sleepCmd("30")
	if err := cmd2.Start(); err != nil {
		t.Fatalf("start cmd2: %v", err)
	}
	procMu.Lock()
	p = procOf(id)
	p.gen = 2
	p.cmd = cmd2
	p.started = true
	p.autoRestart = true
	procMu.Unlock()

	wg2 := sync.WaitGroup{}
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		waitForProcessAndHandleExit(id, cmd2, 2)
	}()

	// 旧 waiter 应只回收 cmd1, 且不得把 cmd2 杀掉/重启
	done1 := make(chan struct{})
	go func() {
		wg.Wait()
		close(done1)
	}()
	select {
	case <-done1:
	case <-time.After(3 * time.Second):
		t.Fatal("waiter1 未在超时内结束(可能卡在 Wait 新进程)")
	}

	procMu.Lock()
	stillCmd := procOf(id).cmd
	stillGen := procOf(id).gen
	stillStarted := procOf(id).started
	procMu.Unlock()
	if stillGen != 2 || !stillStarted || stillCmd != cmd2 {
		t.Fatalf("第二代状态被旧 waiter 破坏: gen=%d started=%v cmd==cmd2=%v", stillGen, stillStarted, stillCmd == cmd2)
	}
	if cmd2.ProcessState != nil {
		t.Fatal("cmd2 不应被旧 waiter Wait 掉")
	}

	// 停第二代, 确认其 waiter 能正常回收
	stopServiceProcess(id)
	done2 := make(chan struct{})
	go func() {
		wg2.Wait()
		close(done2)
	}()
	select {
	case <-done2:
	case <-time.After(3 * time.Second):
		t.Fatal("waiter2 未在超时内结束")
	}
}
