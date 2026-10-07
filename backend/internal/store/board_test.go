package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"starry/backend/internal/model"
)

// TestBuildBoardTreeNeverEmitsNull 是看板白屏事故的回归测试。
//
// 事故原因：空切片被编码成 JSON null，前端 `col.tasks.length` 在 null 上崩溃，
// 导致整个看板页面白屏。因此这里断言的不是一个「好不好看」的细节，而是
// API 契约：columns 与 tasks 无论有无数据都必须是数组 []，绝不能是 null。
func TestBuildBoardTreeNeverEmitsNull(t *testing.T) {
	boardID := uuid.New()
	colID := uuid.New()
	userID := uuid.New()

	cases := []struct {
		name     string
		boards   []model.Board
		columns  []model.BoardColumn
		tasks    []model.BoardTask
		wantNull []string // 这些子串（JSON null 形式）绝不允许出现
	}{
		{
			name:     "完全无数据",
			boards:   nil,
			columns:  nil,
			tasks:    nil,
			wantNull: []string{`"columns":null`, `"tasks":null`},
		},
		{
			name:     "有看板但无列",
			boards:   []model.Board{{ID: boardID, UserID: userID, Name: "空看板"}},
			columns:  nil,
			tasks:    nil,
			wantNull: []string{`"columns":null`, `"tasks":null`},
		},
		{
			name:    "有列但无任务",
			boards:  []model.Board{{ID: boardID, UserID: userID, Name: "看板"}},
			columns: []model.BoardColumn{{ID: colID, BoardID: boardID, UserID: userID, Title: "待办"}},
			tasks:   nil,
			// 该场景 columns 有值，但列内 tasks 必须仍为 []
			wantNull: []string{`"tasks":null`, `"columns":null`},
		},
		{
			name:    "有任务",
			boards:  []model.Board{{ID: boardID, UserID: userID, Name: "看板"}},
			columns: []model.BoardColumn{{ID: colID, BoardID: boardID, UserID: userID, Title: "待办"}},
			tasks:   []model.BoardTask{{ID: uuid.New(), ColumnID: colID, BoardID: boardID, UserID: userID, Title: "任务"}},
			// 有值场景同样不允许退化成 null
			wantNull: []string{`"tasks":null`, `"columns":null`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			views := buildBoardTree(tc.boards, tc.columns, tc.tasks)
			b, err := json.Marshal(views)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}
			got := string(b)
			for _, bad := range tc.wantNull {
				if strings.Contains(got, bad) {
					t.Fatalf("不允许输出 %s，实际: %s", bad, got)
				}
			}
			// 顶层必须是数组，不能是 null
			if got == "null" {
				t.Fatalf("顶层不允许为 null")
			}
		})
	}
}

// TestBuildBoardTreeGroupsByBoardAndColumn 验证归组正确：任务挂到所属列，列挂到所属看板。
func TestBuildBoardTreeGroupsByBoardAndColumn(t *testing.T) {
	userID := uuid.New()
	b1, b2 := uuid.New(), uuid.New()
	c1, c2 := uuid.New(), uuid.New()

	views := buildBoardTree(
		[]model.Board{{ID: b1, UserID: userID}, {ID: b2, UserID: userID}},
		[]model.BoardColumn{
			{ID: c1, BoardID: b1, UserID: userID, Title: "待办"},
			{ID: c2, BoardID: b2, UserID: userID, Title: "进行中"},
		},
		[]model.BoardTask{
			{ID: uuid.New(), BoardID: b1, ColumnID: c1, UserID: userID, Title: "t1"},
			{ID: uuid.New(), BoardID: b1, ColumnID: c1, UserID: userID, Title: "t2"},
		},
	)

	if len(views) != 2 {
		t.Fatalf("期望 2 个看板，实际 %d", len(views))
	}
	for _, v := range views {
		if len(v.Columns) != 1 {
			t.Fatalf("看板 %s 期望 1 列，实际 %d", v.Board.Name, len(v.Columns))
		}
		col := v.Columns[0]
		switch v.Board.ID {
		case b1:
			if len(col.Tasks) != 2 {
				t.Fatalf("待办列期望 2 个任务，实际 %d", len(col.Tasks))
			}
		case b2:
			if len(col.Tasks) != 0 {
				t.Fatalf("进行中列应为空数组，实际 %d", len(col.Tasks))
			}
		}
	}
}
