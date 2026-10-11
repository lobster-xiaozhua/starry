package console

import (
	"testing"

	"starry/backend/internal/bootcheck"
)

// 未记录开机自检时返回空列表而不是 nil，保证 JSON 序列化为 [] 而非 null。
func TestModuleStatusFromBootcheckWithoutRecord(t *testing.T) {
	bootcheck.Record(nil)
	got := moduleStatusFromBootcheck()
	if got == nil {
		t.Fatal("moduleStatusFromBootcheck() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("moduleStatusFromBootcheck() = %+v, want empty", got)
	}
}

func TestModuleStatusFromBootcheck(t *testing.T) {
	bootcheck.Record(&bootcheck.Result{
		Modules: []bootcheck.ModuleStatus{
			{Name: "auth", Enabled: true},
			{Name: "console", Enabled: false},
		},
	})

	got := moduleStatusFromBootcheck()
	if len(got) != 2 {
		t.Fatalf("moduleStatusFromBootcheck() len = %d, want 2", len(got))
	}
	if got[0].Name != "auth" || !got[0].Enabled {
		t.Fatalf("first module = %+v, want auth enabled", got[0])
	}
	if got[1].Name != "console" || got[1].Enabled {
		t.Fatalf("second module = %+v, want console disabled", got[1])
	}
}
