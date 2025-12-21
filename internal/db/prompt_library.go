package db

import "time"

type PromptLibrary struct {
	ID        uint      `gorm:"primaryKey"`
	Category  string    `gorm:"size:64;not null;uniqueIndex:idx_prompt_library_category_text"`
	Text      string    `gorm:"size:280;not null;uniqueIndex:idx_prompt_library_category_text"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
