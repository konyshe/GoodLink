package ui2

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestZipEntryBaseRejectsTraversal(t *testing.T) {
	bad := []string{
		"../evil.exe",
		"..\\evil.exe",
		"foo/../../evil.exe",
		"/abs.exe",
		"C:/Windows/evil.exe",
	}
	for _, name := range bad {
		if _, err := zipEntryBase(name); err == nil {
			t.Errorf("zipEntryBase(%q) should fail", name)
		}
	}
	if base, err := zipEntryBase("goodlink-windows-amd64.exe"); err != nil || base != "goodlink-windows-amd64.exe" {
		t.Fatalf("got %q %v", base, err)
	}
}

func TestParseMd5sumFile(t *testing.T) {
	s := "abc123  goodlink-windows-amd64.exe\ndef456 *wintun.dll\n"
	m := parseMd5sumFile(s)
	if m["goodlink-windows-amd64.exe"] != "abc123" || m["wintun.dll"] != "def456" {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestApplyZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "upd.zip")
	target := filepath.Join(dir, "goodlink.exe")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("malware")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyZip(zipPath, target); err == nil {
		t.Fatal("expected path traversal to fail")
	}
}

func TestApplyZipChecksMd5(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "upd.zip")
	target := filepath.Join(dir, "goodlink.exe")
	payload := []byte("new-binary")
	sum := md5.Sum(payload)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("goodlink.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	mw, err := zw.Create("md5sum.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mw.Write([]byte("deadbeef  goodlink.exe\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyZip(zipPath, target); err == nil {
		t.Fatal("expected md5 mismatch to fail")
	}

	buf.Reset()
	zw = zip.NewWriter(&buf)
	w, err = zw.Create("goodlink.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	mw, err = zw.Create("md5sum.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mw.Write([]byte(hex.EncodeToString(sum[:]) + "  goodlink.exe\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyZip(zipPath, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("target not replaced")
	}
}

// buildZip 按条目名构造内存 zip, 供升级包用例复用
func buildZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestApplyZipPrefersSha256(t *testing.T) {
	payload := []byte("new-binary")
	sha := sha256.Sum256(payload)
	shaHex := hex.EncodeToString(sha[:])
	build := func(shaLine, md5Line string) []byte {
		return buildZip(t, map[string][]byte{
			"goodlink.exe":  payload,
			"sha256sum.txt": []byte(shaLine),
			"md5sum.txt":    []byte(md5Line),
		})
	}

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "upd.zip")
	target := filepath.Join(dir, "goodlink.exe")

	// sha256 正确 + md5 故意错误: 若实现误走 md5 分支必然校验失败,
	// 因此成功即证明 sha256 清单被优先采用
	if err := os.WriteFile(zipPath, build(shaHex+"  goodlink.exe\n", "deadbeef  goodlink.exe\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyZip(zipPath, target); err != nil {
		t.Fatalf("expected sha256 to take precedence over md5: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("target not replaced")
	}

	// sha256 不匹配: 校验失败, 且 target 必须保持替换前内容(先校验后替换)
	if err := os.WriteFile(zipPath, build("deadbeef  goodlink.exe\n", "ignored  goodlink.exe\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyZip(zipPath, target); err == nil {
		t.Fatal("expected sha256 mismatch to fail")
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("target must stay untouched when checksum fails")
	}
}

func TestReplaceFileRollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "goodlink.exe")
	src := filepath.Join(dir, "missing.exe")
	if err := os.WriteFile(dest, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// src 不存在: dest 先被挪到 .old, rename/copy 均失败后必须从 .old 恢复
	err := replaceFile(src, dest)
	if err == nil {
		t.Fatal("expected replaceFile to fail with missing src")
	}
	got, rerr := os.ReadFile(dest)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !bytes.Equal(got, []byte("old")) {
		t.Fatalf("dest should be restored from .old, got %q", got)
	}
	if _, serr := os.Stat(dest + ".old"); !os.IsNotExist(serr) {
		t.Fatalf(".old should be consumed by rollback, stat err = %v", serr)
	}
}
