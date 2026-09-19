package dbcore

import (
	"time"

	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
)

const removedRemoteControlTaskResult = "cancelled: remote command execution was removed from Komari Stable"

// cancelPendingRemoteControlTasks is the only post-removal write to the legacy
// exec-task tables. The tables and completed history stay intact for binary
// rollback, while every unfinished request becomes terminal and cannot appear
// pending after an upgrade.
func cancelPendingRemoteControlTasks(db *gorm.DB, now time.Time) (int64, error) {
	exitCode := -1
	result := db.Model(&models.TaskResult{}).
		Where("exit_code IS NULL").
		Updates(map[string]any{
			"result":      removedRemoteControlTaskResult,
			"exit_code":   exitCode,
			"finished_at": now.UTC(),
		})
	return result.RowsAffected, result.Error
}
