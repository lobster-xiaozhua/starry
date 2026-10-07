package model

import (
	"time"

	"github.com/google/uuid"
)

type Note struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Body      string    `gorm:"type:text;not null;default:''" json:"body"`
	Archived  bool      `gorm:"not null;default:false" json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Note) TableName() string { return "notes" }

type NoteWithTags struct {
	Note
	Tags []string `json:"tags"`
}

type Tag struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;index" json:"-"`
	Name   string    `gorm:"size:128;not null" json:"name"`
}

func (Tag) TableName() string { return "tags" }

type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type NoteTag struct {
	NoteID uuid.UUID `gorm:"type:uuid" json:"-"`
	TagID  uuid.UUID `gorm:"type:uuid" json:"-"`
}

func (NoteTag) TableName() string { return "note_tags" }

type NoteStableID struct {
	NoteID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	UserID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Stable string    `gorm:"size:128;primaryKey" json:"stableId"`
}

func (NoteStableID) TableName() string { return "note_stable_ids" }

type Attachment struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;index" json:"-"`
	NoteID    *uuid.UUID `gorm:"type:uuid" json:"noteId"`
	Filename  string     `gorm:"size:256;not null" json:"filename"`
	URL       string     `gorm:"size:512;not null" json:"url"`
	ThumbURL  string     `gorm:"size:512" json:"thumbUrl"`
	SizeBytes int64      `json:"sizeBytes"`
	MimeType  string     `gorm:"size:128;not null" json:"mimeType"`
	CreatedAt time.Time  `json:"createdAt"`
}

func (Attachment) TableName() string { return "attachments" }
