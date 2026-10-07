package store

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"login-system/backend/internal/model"
)

// 默认看板列（建板时自动播种）。
var defaultColumnTitles = []string{"待办", "进行中", "已完成"}

// BoardView 是看板的前端视图：看板 + 其下列（含列内任务）。
type BoardView struct {
	model.Board
	Columns []ColumnView `json:"columns"`
}

// ColumnView 是列的视图：列 + 其内任务。
type ColumnView struct {
	model.BoardColumn
	Tasks []model.BoardTask `json:"tasks"`
}

// ListBoardTree 返回某用户的全部看板，并装配好列与任务（按 position 排序）。
func (s *DB) ListBoardTree(userID uuid.UUID) ([]BoardView, error) {
	var boards []model.Board
	if err := s.gorm.Where("user_id = ?", userID).Order("position ASC, created_at ASC").
		Find(&boards).Error; err != nil {
		return nil, err
	}
	var columns []model.BoardColumn
	if err := s.gorm.Where("user_id = ?", userID).Order("position ASC, created_at ASC").
		Find(&columns).Error; err != nil {
		return nil, err
	}
	var tasks []model.BoardTask
	if err := s.gorm.Where("user_id = ?", userID).Order("position ASC, created_at ASC").
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	taskByCol := map[uuid.UUID][]model.BoardTask{}
	for _, t := range tasks {
		taskByCol[t.ColumnID] = append(taskByCol[t.ColumnID], t)
	}
	colsByBoard := map[uuid.UUID][]ColumnView{}
	for _, c := range columns {
		colsByBoard[c.BoardID] = append(colsByBoard[c.BoardID], ColumnView{
			BoardColumn: c, Tasks: taskByCol[c.ID],
		})
	}

	views := make([]BoardView, 0, len(boards))
	for _, b := range boards {
		views = append(views, BoardView{Board: b, Columns: colsByBoard[b.ID]})
	}
	return views, nil
}

// CreateBoard 创建看板并播种默认三列（事务内完成）。
func (s *DB) CreateBoard(userID uuid.UUID, name, color string) (*model.Board, error) {
	if color == "" {
		color = "#22c55e"
	}
	var board model.Board
	err := s.gorm.Transaction(func(tx *gorm.DB) error {
		board = model.Board{
			ID:       uuid.New(),
			UserID:   userID,
			Name:     name,
			Color:    color,
			Position: 0,
		}
		if err := tx.Create(&board).Error; err != nil {
			return err
		}
		// 取当前用户看板数作为新看板 position
		var cnt int64
		if err := tx.Model(&model.Board{}).Where("user_id = ?", userID).Count(&cnt).Error; err != nil {
			return err
		}
		board.Position = int(cnt) - 1
		if err := tx.Model(&board).Update("position", board.Position).Error; err != nil {
			return err
		}
		for i, title := range defaultColumnTitles {
			col := model.BoardColumn{
				ID:       uuid.New(),
				BoardID:  board.ID,
				UserID:   userID,
				Title:    title,
				Position: i,
			}
			if err := tx.Create(&col).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &board, nil
}

// UpdateBoard 更新看板名称/颜色/排序。
func (s *DB) UpdateBoard(userID, boardID uuid.UUID, name, color *string, position *int) error {
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if color != nil {
		updates["color"] = *color
	}
	if position != nil {
		updates["position"] = *position
	}
	if len(updates) == 0 {
		return nil
	}
	return s.gorm.Model(&model.Board{}).Where("user_id = ? AND id = ?", userID, boardID).
		Updates(updates).Error
}

// DeleteBoard 删除看板及其列与任务（事务级联）。
func (s *DB) DeleteBoard(userID, boardID uuid.UUID) error {
	return s.gorm.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND board_id = ?", userID, boardID).
			Delete(&model.BoardTask{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND board_id = ?", userID, boardID).
			Delete(&model.BoardColumn{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND id = ?", userID, boardID).
			Delete(&model.Board{}).Error
	})
}

// CreateColumn 在看板下新建一列。
func (s *DB) CreateColumn(userID, boardID uuid.UUID, title string) (*model.BoardColumn, error) {
	var cnt int64
	if err := s.gorm.Model(&model.BoardColumn{}).Where("board_id = ?", boardID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	col := model.BoardColumn{
		ID:       uuid.New(),
		BoardID:  boardID,
		UserID:   userID,
		Title:    title,
		Position: int(cnt),
	}
	if err := s.gorm.Create(&col).Error; err != nil {
		return nil, err
	}
	return &col, nil
}

// UpdateColumn 更新列标题/排序。
func (s *DB) UpdateColumn(userID, colID uuid.UUID, title *string, position *int) error {
	updates := map[string]interface{}{}
	if title != nil {
		updates["title"] = *title
	}
	if position != nil {
		updates["position"] = *position
	}
	if len(updates) == 0 {
		return nil
	}
	return s.gorm.Model(&model.BoardColumn{}).Where("user_id = ? AND id = ?", userID, colID).
		Updates(updates).Error
}

// DeleteColumn 删除列（同时清空其任务）。
func (s *DB) DeleteColumn(userID, colID uuid.UUID) error {
	return s.gorm.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND column_id = ?", userID, colID).
			Delete(&model.BoardTask{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND id = ?", userID, colID).
			Delete(&model.BoardColumn{}).Error
	})
}

// CreateBoardTask 在看板列下新建任务卡片。
func (s *DB) CreateBoardTask(t *model.BoardTask) (*model.BoardTask, error) {
	if t.Priority == "" {
		t.Priority = "medium"
	}
	var cnt int64
	if err := s.gorm.Model(&model.BoardTask{}).Where("column_id = ?", t.ColumnID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	t.Position = int(cnt)
	t.ID = uuid.New()
	if err := s.gorm.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTask 更新任务字段（含移动到其他列/排序）。
func (s *DB) UpdateTask(userID, taskID uuid.UUID, patch map[string]interface{}) error {
	if len(patch) == 0 {
		return nil
	}
	return s.gorm.Model(&model.BoardTask{}).Where("user_id = ? AND id = ?", userID, taskID).
		Updates(patch).Error
}

// DeleteTask 删除单张任务卡片。
func (s *DB) DeleteTask(userID, taskID uuid.UUID) error {
	return s.gorm.Where("user_id = ? AND id = ?", userID, taskID).
		Delete(&model.BoardTask{}).Error
}
