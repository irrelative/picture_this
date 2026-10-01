-- These constraints were previously supplied only by GORM AutoMigrate.
CREATE UNIQUE INDEX IF NOT EXISTS uni_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
