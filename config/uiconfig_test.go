package config

import "testing"

func TestNormalizeAndValidate(t *testing.T) {
	cfg := UIConfig{
		LocalMode: "FORWARD",
		ForwardRules: []UIForwardRule{
			{Proto: "TCP", Listen: "127.0.0.1:3389", Remote: "127.0.0.1:3389"},
			{Proto: "udp", Listen: "0.0.0.0:53", Remote: "8.8.8.8:53"},
		},
	}
	if err := NormalizeAndValidate(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.LocalMode != LocalModeForward {
		t.Fatalf("mode=%s", cfg.LocalMode)
	}
	if cfg.ForwardRules[0].Proto != "tcp" || cfg.ForwardRules[0].Listen != "127.0.0.1:3389" {
		t.Fatalf("rule0=%+v", cfg.ForwardRules[0])
	}

	dup := UIConfig{
		LocalMode: "forward",
		ForwardRules: []UIForwardRule{
			{Proto: "tcp", Listen: "127.0.0.1:22", Remote: "127.0.0.1:22"},
			{Proto: "tcp", Listen: "127.0.0.1:22", Remote: "192.168.1.1:22"},
		},
	}
	if err := NormalizeAndValidate(&dup); err == nil {
		t.Fatal("expected duplicate listen error")
	}

	badIP := UIConfig{
		LocalMode: "forward",
		ForwardRules: []UIForwardRule{
			{Proto: "tcp", Listen: "localhost:22", Remote: "127.0.0.1:22"},
		},
	}
	if err := NormalizeAndValidate(&badIP); err == nil {
		t.Fatal("expected ipv4 error")
	}

	empty := UIConfig{}
	if err := NormalizeAndValidate(&empty); err != nil {
		t.Fatal(err)
	}
	if empty.LocalMode != LocalModeTUN {
		t.Fatalf("default mode=%s", empty.LocalMode)
	}
}
