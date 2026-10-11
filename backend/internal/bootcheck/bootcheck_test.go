package bootcheck

import (
	"testing"
	"time"
)

func TestRecordAndGet(t *testing.T) {
	Record(nil)
	if got := Get(); got != nil {
		t.Fatalf("Get() after Record(nil) = %+v, want nil", got)
	}

	r := &Result{
		Timestamp: time.Now(),
		AppEnv:    "test",
		Version:   "dev",
		Checks:    []Check{{Name: "postgres", Status: "ok"}, {Name: "orphan_cleanup", Status: "ok", Count: 3}},
		Modules:   []ModuleStatus{{Name: "auth", Enabled: true}},
	}
	Record(r)
	if got := Get(); got != r {
		t.Fatalf("Get() = %+v, want the recorded result", got)
	}

	// 重复 Record 覆盖旧结果（重启/二次自检场景）。
	r2 := &Result{AppEnv: "prod"}
	Record(r2)
	if got := Get(); got != r2 {
		t.Fatalf("Get() after second Record = %+v, want the latest result", got)
	}
}
