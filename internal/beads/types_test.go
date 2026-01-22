package beads

import (
	"testing"
	"time"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{"open status", StatusOpen, "open"},
		{"in_progress status", StatusInProgress, "in_progress"},
		{"blocked status", StatusBlocked, "blocked"},
		{"closed status", StatusClosed, "closed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Status constant %v = %q, want %q", tt.name, tt.status, tt.expected)
			}
		})
	}
}

func TestPriorityConstants(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		expected string
	}{
		{"p0 critical", PriorityP0, "p0"},
		{"p1 high", PriorityP1, "p1"},
		{"p2 medium", PriorityP2, "p2"},
		{"p3 low", PriorityP3, "p3"},
		{"p4 very low", PriorityP4, "p4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.priority) != tt.expected {
				t.Errorf("Priority constant %v = %q, want %q", tt.name, tt.priority, tt.expected)
			}
		})
	}
}

func TestIssueStructure(t *testing.T) {
	t.Run("minimal issue", func(t *testing.T) {
		now := time.Now()
		issue := Issue{
			ID:       "test-1",
			Title:    "Test Issue",
			Status:   StatusOpen,
			Priority: PriorityP2,
			Created:  now,
			Updated:  now,
		}

		if issue.ID != "test-1" {
			t.Errorf("ID = %q, want %q", issue.ID, "test-1")
		}
		if issue.Title != "Test Issue" {
			t.Errorf("Title = %q, want %q", issue.Title, "Test Issue")
		}
		if issue.Status != StatusOpen {
			t.Errorf("Status = %q, want %q", issue.Status, StatusOpen)
		}
		if issue.Priority != PriorityP2 {
			t.Errorf("Priority = %q, want %q", issue.Priority, PriorityP2)
		}
	})

	t.Run("issue with all fields", func(t *testing.T) {
		now := time.Now()
		issue := Issue{
			ID:          "test-2",
			Title:       "Complete Issue",
			Description: "This is a detailed description",
			Status:      StatusInProgress,
			Priority:    PriorityP1,
			Epic:        "epic-1",
			Assignee:    "user@example.com",
			Labels:      []string{"bug", "frontend"},
			DependsOn:   []string{"test-1"},
			Created:     now,
			Updated:     now.Add(time.Hour),
			Metadata: Metadata{
				JiraKey:       "PROJ-123",
				JiraID:        "12345",
				JiraIssueType: "Bug",
				Custom: map[string]string{
					"team":       "frontend",
					"repository": "main-app",
				},
			},
		}

		if len(issue.Labels) != 2 {
			t.Errorf("Expected 2 labels, got %d", len(issue.Labels))
		}
		if len(issue.DependsOn) != 1 {
			t.Errorf("Expected 1 dependency, got %d", len(issue.DependsOn))
		}
		if len(issue.Metadata.Custom) != 2 {
			t.Errorf("Expected 2 custom metadata fields, got %d", len(issue.Metadata.Custom))
		}
	})

	t.Run("issue with empty optional fields", func(t *testing.T) {
		now := time.Now()
		issue := Issue{
			ID:       "test-3",
			Title:    "Minimal",
			Status:   StatusOpen,
			Priority: PriorityP2,
			Created:  now,
			Updated:  now,
		}

		if issue.Description != "" {
			t.Error("Expected empty description")
		}
		if issue.Epic != "" {
			t.Error("Expected empty epic")
		}
		if issue.Assignee != "" {
			t.Error("Expected empty assignee")
		}
		if issue.Labels != nil {
			t.Error("Expected nil labels")
		}
		if issue.DependsOn != nil {
			t.Error("Expected nil dependencies")
		}
	})
}

func TestEpicStructure(t *testing.T) {
	t.Run("minimal epic", func(t *testing.T) {
		now := time.Now()
		epic := Epic{
			ID:      "epic-1",
			Name:    "Test Epic",
			Status:  StatusOpen,
			Created: now,
			Updated: now,
		}

		if epic.ID != "epic-1" {
			t.Errorf("ID = %q, want %q", epic.ID, "epic-1")
		}
		if epic.Name != "Test Epic" {
			t.Errorf("Name = %q, want %q", epic.Name, "Test Epic")
		}
		if epic.Status != StatusOpen {
			t.Errorf("Status = %q, want %q", epic.Status, StatusOpen)
		}
	})

	t.Run("epic with all fields", func(t *testing.T) {
		now := time.Now()
		epic := Epic{
			ID:          "epic-2",
			Name:        "Complete Epic",
			Description: "Epic description",
			Status:      StatusInProgress,
			Created:     now,
			Updated:     now.Add(24 * time.Hour),
			Metadata: Metadata{
				JiraKey:       "EPIC-1",
				JiraID:        "98765",
				JiraIssueType: "Epic",
			},
		}

		if epic.Description != "Epic description" {
			t.Errorf("Description = %q, want %q", epic.Description, "Epic description")
		}
		if epic.Metadata.JiraKey != "EPIC-1" {
			t.Errorf("JiraKey = %q, want %q", epic.Metadata.JiraKey, "EPIC-1")
		}
	})
}

func TestMetadataStructure(t *testing.T) {
	t.Run("empty metadata", func(t *testing.T) {
		metadata := Metadata{}

		if metadata.JiraKey != "" {
			t.Error("Expected empty JiraKey")
		}
		if metadata.JiraID != "" {
			t.Error("Expected empty JiraID")
		}
		if metadata.JiraIssueType != "" {
			t.Error("Expected empty JiraIssueType")
		}
		if metadata.Custom != nil {
			t.Error("Expected nil Custom map")
		}
	})

	t.Run("metadata with Jira fields", func(t *testing.T) {
		metadata := Metadata{
			JiraKey:       "PROJ-456",
			JiraID:        "54321",
			JiraIssueType: "Story",
		}

		if metadata.JiraKey != "PROJ-456" {
			t.Errorf("JiraKey = %q, want %q", metadata.JiraKey, "PROJ-456")
		}
		if metadata.JiraID != "54321" {
			t.Errorf("JiraID = %q, want %q", metadata.JiraID, "54321")
		}
		if metadata.JiraIssueType != "Story" {
			t.Errorf("JiraIssueType = %q, want %q", metadata.JiraIssueType, "Story")
		}
	})

	t.Run("metadata with custom fields", func(t *testing.T) {
		metadata := Metadata{
			Custom: map[string]string{
				"repository": "backend-api",
				"team":       "platform",
				"sprint":     "23",
			},
		}

		if len(metadata.Custom) != 3 {
			t.Errorf("Expected 3 custom fields, got %d", len(metadata.Custom))
		}
		if metadata.Custom["repository"] != "backend-api" {
			t.Errorf("repository = %q, want %q", metadata.Custom["repository"], "backend-api")
		}
	})
}

func TestExportStructure(t *testing.T) {
	t.Run("empty export", func(t *testing.T) {
		export := Export{}

		if export.Issues != nil {
			t.Error("Expected nil Issues")
		}
		if export.Epics != nil {
			t.Error("Expected nil Epics")
		}
	})

	t.Run("export with issues and epics", func(t *testing.T) {
		now := time.Now()
		export := Export{
			Issues: []Issue{
				{
					ID:       "issue-1",
					Title:    "Issue 1",
					Status:   StatusOpen,
					Priority: PriorityP2,
					Created:  now,
					Updated:  now,
				},
				{
					ID:       "issue-2",
					Title:    "Issue 2",
					Status:   StatusClosed,
					Priority: PriorityP3,
					Created:  now,
					Updated:  now,
				},
			},
			Epics: []Epic{
				{
					ID:      "epic-1",
					Name:    "Epic 1",
					Status:  StatusOpen,
					Created: now,
					Updated: now,
				},
			},
		}

		if len(export.Issues) != 2 {
			t.Errorf("Expected 2 issues, got %d", len(export.Issues))
		}
		if len(export.Epics) != 1 {
			t.Errorf("Expected 1 epic, got %d", len(export.Epics))
		}
	})
}

func TestStatusValidation(t *testing.T) {
	validStatuses := []Status{
		StatusOpen,
		StatusInProgress,
		StatusBlocked,
		StatusClosed,
	}

	// Test that all valid statuses are distinct
	seen := make(map[Status]bool)
	for _, status := range validStatuses {
		if seen[status] {
			t.Errorf("Duplicate status value: %v", status)
		}
		seen[status] = true
	}

	// Ensure we have exactly 4 distinct statuses
	if len(seen) != 4 {
		t.Errorf("Expected 4 distinct statuses, got %d", len(seen))
	}
}

func TestPriorityValidation(t *testing.T) {
	validPriorities := []Priority{
		PriorityP0,
		PriorityP1,
		PriorityP2,
		PriorityP3,
		PriorityP4,
	}

	// Test that all valid priorities are distinct
	seen := make(map[Priority]bool)
	for _, priority := range validPriorities {
		if seen[priority] {
			t.Errorf("Duplicate priority value: %v", priority)
		}
		seen[priority] = true
	}

	// Ensure we have exactly 5 distinct priorities
	if len(seen) != 5 {
		t.Errorf("Expected 5 distinct priorities, got %d", len(seen))
	}
}

func TestPriorityOrdering(t *testing.T) {
	// Test that priorities follow the expected order
	priorities := []Priority{PriorityP0, PriorityP1, PriorityP2, PriorityP3, PriorityP4}

	expectedOrder := []string{"p0", "p1", "p2", "p3", "p4"}

	for i, priority := range priorities {
		if string(priority) != expectedOrder[i] {
			t.Errorf("At index %d: expected %q, got %q", i, expectedOrder[i], priority)
		}
	}
}

func TestCustomMetadataEdgeCases(t *testing.T) {
	t.Run("empty string values", func(t *testing.T) {
		metadata := Metadata{
			Custom: map[string]string{
				"key1": "",
				"key2": "value",
			},
		}

		if metadata.Custom["key1"] != "" {
			t.Error("Empty string value should be preserved")
		}
	})

	t.Run("special characters in keys and values", func(t *testing.T) {
		metadata := Metadata{
			Custom: map[string]string{
				"key-with-dash":       "value",
				"key.with.dots":       "value",
				"key_with_underscore": "value-with-special-chars!@#",
			},
		}

		if len(metadata.Custom) != 3 {
			t.Errorf("Expected 3 custom fields, got %d", len(metadata.Custom))
		}
	})

	t.Run("unicode in custom fields", func(t *testing.T) {
		metadata := Metadata{
			Custom: map[string]string{
				"language": "日本語",
				"emoji":    "🚀",
			},
		}

		if metadata.Custom["language"] != "日本語" {
			t.Error("Unicode values should be preserved")
		}
		if metadata.Custom["emoji"] != "🚀" {
			t.Error("Emoji values should be preserved")
		}
	})
}
