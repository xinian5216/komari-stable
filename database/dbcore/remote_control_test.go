package dbcore

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCancelPendingRemoteControlTasksPreservesHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:cancel_remote_tasks?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Client{}, &models.Task{}, &models.TaskResult{}); err != nil {
		t.Fatalf("migrate task tables: %v", err)
	}
	if err := db.Create(&[]models.Client{
		{UUID: "node-a", Token: "token-node-a"},
		{UUID: "node-b", Token: "token-node-b"},
	}).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := db.Create(&[]models.Task{
		{TaskId: "pending", Command: "old-pending-command"},
		{TaskId: "complete", Command: "old-completed-command"},
	}).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}

	completedCode := 0
	completedAt := time.Date(2026, 9, 18, 1, 2, 3, 0, time.UTC)
	rows := []models.TaskResult{
		{TaskId: "pending", Client: "node-a", Result: "", ExitCode: nil},
		{TaskId: "complete", Client: "node-b", Result: "kept", ExitCode: &completedCode, FinishedAt: &completedAt},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create task results: %v", err)
	}

	now := time.Date(2026, 9, 19, 2, 0, 0, 0, time.UTC)
	count, err := cancelPendingRemoteControlTasks(db, now)
	if err != nil {
		t.Fatalf("cancel pending tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("cancelled rows = %d, want 1", count)
	}

	var pending, complete models.TaskResult
	if err := db.Where("task_id = ?", "pending").First(&pending).Error; err != nil {
		t.Fatalf("load pending row: %v", err)
	}
	if pending.ExitCode == nil || *pending.ExitCode != -1 || pending.Result != removedRemoteControlTaskResult || pending.FinishedAt == nil || !pending.FinishedAt.Equal(now) {
		t.Fatalf("pending row was not cancelled: %#v", pending)
	}
	if err := db.Where("task_id = ?", "complete").First(&complete).Error; err != nil {
		t.Fatalf("load completed row: %v", err)
	}
	if complete.ExitCode == nil || *complete.ExitCode != 0 || complete.Result != "kept" || complete.FinishedAt == nil || !complete.FinishedAt.Equal(completedAt) {
		t.Fatalf("completed history changed: %#v", complete)
	}
}
