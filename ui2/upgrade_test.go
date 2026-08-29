package ui2

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
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
