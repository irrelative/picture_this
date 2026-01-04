package db

import "time"

type PromptLibrary struct {
	ID            uint      `gorm:"primaryKey"`
	Text          string    `gorm:"size:280;not null;uniqueIndex:idx_prompt_library_text"`
	Joke          string    `gorm:"size:280"`
	JokeAudioPath string    `gorm:"size:280"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}
