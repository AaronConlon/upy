package log

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestLogSetOutputAndRedact(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	RegisterSecret("secret-token-12345")

	Info("testing message with secret-token-12345 inside")
	out := buf.String()
	if strings.Contains(out, "secret-token-12345") {
		t.Fatalf("敏感值未被脱敏: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("未包含脱敏星号: %s", out)
	}
}

func TestLogConcurrency(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			RegisterSecret("token")
			Step("step")
			Ok("ok")
			Info("info")
		}(i)
	}
	wg.Wait()
}
