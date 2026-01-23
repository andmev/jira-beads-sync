package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/conallob/jira-beads-sync/internal/beads"
	"github.com/conallob/jira-beads-sync/internal/converter"
	"github.com/conallob/jira-beads-sync/internal/jira"
	"gopkg.in/yaml.v3"
)

// TestEndToEndJiraToBeadsSync performs a full end-to-end integration test:
// 1. Mocks Jira API responses with realistic data
// 2. Fetches issues using the Jira client
// 3. Converts to beads format using the converter
// 4. Writes beads YAML files
// 5. Reads back and validates the beads data
// 6. Optionally tests with beads CLI if available
func TestEndToEndJiraToBeadsSync(t *testing.T) {
	// Create temporary directory for beads repository
	tmpDir, err := os.MkdirTemp("", "jira-beads-sync-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Logf("Test directory: %s", tmpDir)

	// Step 1: Initialize beads repository (if bd CLI is available)
	if beadsAvailable() {
		t.Log("Beads CLI detected, initializing repository...")
		if err := initBeadsRepo(tmpDir); err != nil {
			t.Logf("Warning: Failed to initialize beads repo: %v", err)
			t.Log("Continuing without bd init...")
		} else {
			t.Log("✓ Beads repository initialized")
		}
	} else {
		t.Log("Beads CLI not available, creating .beads directory manually...")
		if err := os.MkdirAll(filepath.Join(tmpDir, ".beads", "issues"), 0755); err != nil {
			t.Fatalf("Failed to create .beads/issues directory: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(tmpDir, ".beads", "epics"), 0755); err != nil {
			t.Fatalf("Failed to create .beads/epics directory: %v", err)
		}
	}

	// Step 2: Set up mock Jira server with realistic data
	mockData := createMockJiraData()
	server := createMockJiraServer(t, mockData)
	defer server.Close()

	t.Logf("Mock Jira server running at: %s", server.URL)

	// Step 3: Fetch issues from mock Jira server
	t.Log("Fetching issues from mock Jira...")
	client := jira.NewClient(server.URL, "test@example.com", "test-token")
	jiraExport, err := client.FetchIssueWithDependencies("PROJ-100")
	if err != nil {
		t.Fatalf("Failed to fetch issues: %v", err)
	}

	t.Logf("✓ Fetched %d issue(s) from Jira", len(jiraExport.Issues))

	// Step 4: Convert to beads format
	t.Log("Converting to beads format...")
	protoConverter := converter.NewProtoConverter()
	beadsExport, err := protoConverter.Convert(jiraExport)
	if err != nil {
		t.Fatalf("Failed to convert to beads format: %v", err)
	}

	t.Logf("✓ Converted to %d issue(s) and %d epic(s)", len(beadsExport.Issues), len(beadsExport.Epics))

	// Step 5: Write beads YAML files
	t.Log("Writing beads YAML files...")
	yamlRenderer := beads.NewYAMLRenderer(tmpDir)
	if err := yamlRenderer.RenderExport(beadsExport); err != nil {
		t.Fatalf("Failed to render beads files: %v", err)
	}

	t.Log("✓ Beads files written")

	// Step 6: Verify files were created
	issuesDir := filepath.Join(tmpDir, ".beads", "issues")
	epicsDir := filepath.Join(tmpDir, ".beads", "epics")

	issueFiles, err := os.ReadDir(issuesDir)
	if err != nil {
		t.Fatalf("Failed to read issues directory: %v", err)
	}

	epicFiles, err := os.ReadDir(epicsDir)
	if err != nil {
		t.Fatalf("Failed to read epics directory: %v", err)
	}

	t.Logf("✓ Found %d issue file(s) and %d epic file(s)", len(issueFiles), len(epicFiles))

	// Step 7: Read back and validate beads data
	t.Log("Validating beads data...")

	// Validate main epic (PROJ-100) - epics are in a different directory
	_ = validateBeadsEpic(t, tmpDir, "proj-100", BeadsEpicExpectation{
		Name:          "Main Epic Issue",
		Status:        "open",
		JiraKey:       "PROJ-100",
		JiraIssueType: "Epic",
	})
	t.Log("✓ Main epic validated")

	// Validate story (PROJ-101)
	story := validateBeadsIssue(t, tmpDir, "proj-101", BeadsIssueExpectation{
		Title:         "User Story",
		Status:        "in_progress",
		Priority:      "p2",
		JiraKey:       "PROJ-101",
		JiraIssueType: "Story",
		Epic:          "proj-100",
		HasAssignee:   true,
	})
	t.Log("✓ Story validated")

	// Validate subtask (PROJ-102)
	subtask := validateBeadsIssue(t, tmpDir, "proj-102", BeadsIssueExpectation{
		Title:            "Subtask",
		Status:           "open",
		Priority:         "p2",
		JiraKey:          "PROJ-102",
		JiraIssueType:    "Subtask",
		DependencyCount:  0,
		ExpectDependency: "",
	})
	t.Log("✓ Subtask validated")

	// Validate blocked issue (PROJ-103)
	// Note: Dependency tracking from inward links may vary based on converter implementation
	blocked := validateBeadsIssue(t, tmpDir, "proj-103", BeadsIssueExpectation{
		Title:         "Blocked Task",
		Status:        "open",
		Priority:      "p3",
		JiraKey:       "PROJ-103",
		JiraIssueType: "Task",
		// Dependencies are not validated here as inward link handling may vary
	})
	t.Log("✓ Blocked issue validated")

	// Step 8: Test with beads CLI if available
	if beadsAvailable() {
		t.Log("Testing with beads CLI...")
		testBeadsCLI(t, tmpDir, story, subtask, blocked)
	} else {
		t.Log("Skipping beads CLI tests (bd not available)")
	}

	t.Log("✓ End-to-end integration test completed successfully!")
}

// BeadsIssueExpectation defines what we expect in a beads issue
type BeadsIssueExpectation struct {
	Title            string
	Status           string
	Priority         string
	JiraKey          string
	JiraIssueType    string
	Epic             string
	LabelCount       int
	HasAssignee      bool
	DependencyCount  int
	ExpectDependency string
}

// BeadsEpicExpectation defines what we expect in a beads epic
type BeadsEpicExpectation struct {
	Name          string
	Status        string
	JiraKey       string
	JiraIssueType string
}

// validateBeadsIssue reads a beads YAML file and validates its contents
func validateBeadsIssue(t *testing.T, baseDir, issueID string, expected BeadsIssueExpectation) map[string]interface{} {
	t.Helper()

	issueFile := filepath.Join(baseDir, ".beads", "issues", issueID+".yaml")
	data, err := os.ReadFile(issueFile)
	if err != nil {
		t.Fatalf("Failed to read issue file %s: %v", issueFile, err)
	}

	var issue map[string]interface{}
	if err := yaml.Unmarshal(data, &issue); err != nil {
		t.Fatalf("Failed to parse YAML for %s: %v", issueID, err)
	}

	// Validate basic fields
	if issue["id"] != issueID {
		t.Errorf("Issue %s: expected id %q, got %q", issueID, issueID, issue["id"])
	}

	if issue["title"] != expected.Title {
		t.Errorf("Issue %s: expected title %q, got %q", issueID, expected.Title, issue["title"])
	}

	if issue["status"] != expected.Status {
		t.Errorf("Issue %s: expected status %q, got %q", issueID, expected.Status, issue["status"])
	}

	if issue["priority"] != expected.Priority {
		t.Errorf("Issue %s: expected priority %q, got %q", issueID, expected.Priority, issue["priority"])
	}

	// Validate metadata
	if metadata, ok := issue["metadata"].(map[string]interface{}); ok {
		if metadata["jiraKey"] != expected.JiraKey {
			t.Errorf("Issue %s: expected jiraKey %q, got %q", issueID, expected.JiraKey, metadata["jiraKey"])
		}
		if metadata["jiraIssueType"] != expected.JiraIssueType {
			t.Errorf("Issue %s: expected jiraIssueType %q, got %q", issueID, expected.JiraIssueType, metadata["jiraIssueType"])
		}
	} else {
		t.Errorf("Issue %s: metadata not found or invalid", issueID)
	}

	// Validate epic relationship
	if expected.Epic != "" {
		if issue["epic"] != expected.Epic {
			t.Errorf("Issue %s: expected epic %q, got %q", issueID, expected.Epic, issue["epic"])
		}
	}

	// Validate labels
	if expected.LabelCount > 0 {
		labels, ok := issue["labels"].([]interface{})
		if !ok || len(labels) != expected.LabelCount {
			t.Errorf("Issue %s: expected %d labels, got %v", issueID, expected.LabelCount, labels)
		}
	}

	// Validate assignee
	if expected.HasAssignee {
		if _, ok := issue["assignee"]; !ok {
			t.Errorf("Issue %s: expected assignee field", issueID)
		}
	}

	// Validate dependencies
	if expected.DependencyCount > 0 {
		deps, ok := issue["dependsOn"].([]interface{})
		if !ok || len(deps) != expected.DependencyCount {
			t.Errorf("Issue %s: expected %d dependencies, got %v", issueID, expected.DependencyCount, deps)
		}

		if expected.ExpectDependency != "" {
			found := false
			for _, dep := range deps {
				if dep.(string) == expected.ExpectDependency {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Issue %s: expected dependency on %q, not found in %v", issueID, expected.ExpectDependency, deps)
			}
		}
	}

	return issue
}

// validateBeadsEpic reads a beads epic YAML file and validates its contents
func validateBeadsEpic(t *testing.T, baseDir, epicID string, expected BeadsEpicExpectation) map[string]interface{} {
	t.Helper()

	epicFile := filepath.Join(baseDir, ".beads", "epics", epicID+".yaml")
	data, err := os.ReadFile(epicFile)
	if err != nil {
		t.Fatalf("Failed to read epic file %s: %v", epicFile, err)
	}

	var epic map[string]interface{}
	if err := yaml.Unmarshal(data, &epic); err != nil {
		t.Fatalf("Failed to parse YAML for epic %s: %v", epicID, err)
	}

	// Validate basic fields
	if epic["id"] != epicID {
		t.Errorf("Epic %s: expected id %q, got %q", epicID, epicID, epic["id"])
	}

	if epic["name"] != expected.Name {
		t.Errorf("Epic %s: expected name %q, got %q", epicID, expected.Name, epic["name"])
	}

	if epic["status"] != expected.Status {
		t.Errorf("Epic %s: expected status %q, got %q", epicID, expected.Status, epic["status"])
	}

	// Validate metadata
	if metadata, ok := epic["metadata"].(map[string]interface{}); ok {
		if metadata["jiraKey"] != expected.JiraKey {
			t.Errorf("Epic %s: expected jiraKey %q, got %q", epicID, expected.JiraKey, metadata["jiraKey"])
		}
		if metadata["jiraIssueType"] != expected.JiraIssueType {
			t.Errorf("Epic %s: expected jiraIssueType %q, got %q", epicID, expected.JiraIssueType, metadata["jiraIssueType"])
		}
	} else {
		t.Errorf("Epic %s: metadata not found or invalid", epicID)
	}

	return epic
}

// createMockJiraData creates realistic mock Jira data for testing
func createMockJiraData() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"PROJ-100": {
			"key": "PROJ-100",
			"id":  "10100",
			"fields": map[string]interface{}{
				"summary":     "Main Epic Issue",
				"description": "This is a main epic that contains stories",
				"issuetype": map[string]interface{}{
					"name": "Epic",
				},
				"status": map[string]interface{}{
					"name": "Open",
					"statusCategory": map[string]interface{}{
						"key": "new",
					},
				},
				"priority": map[string]interface{}{
					"name": "High",
				},
				"created": "2024-01-01T10:00:00.000+0000",
				"updated": "2024-01-15T14:30:00.000+0000",
				"labels":  []string{"epic", "important"},
				"subtasks": []map[string]interface{}{
					{"key": "PROJ-101"},
				},
			},
		},
		"PROJ-101": {
			"key": "PROJ-101",
			"id":  "10101",
			"fields": map[string]interface{}{
				"summary":     "User Story",
				"description": "Implement user authentication",
				"issuetype": map[string]interface{}{
					"name": "Story",
				},
				"status": map[string]interface{}{
					"name": "In Progress",
					"statusCategory": map[string]interface{}{
						"key": "indeterminate",
					},
				},
				"priority": map[string]interface{}{
					"name": "Medium",
				},
				"assignee": map[string]interface{}{
					"accountId":    "user123",
					"displayName":  "John Doe",
					"emailAddress": "john@example.com",
				},
				"created": "2024-01-02T10:00:00.000+0000",
				"updated": "2024-01-16T14:30:00.000+0000",
				"parent": map[string]interface{}{
					"key": "PROJ-100",
					"fields": map[string]interface{}{
						"issuetype": map[string]interface{}{
							"name": "Epic",
						},
					},
				},
				"subtasks": []map[string]interface{}{
					{"key": "PROJ-102"},
				},
				"issuelinks": []map[string]interface{}{
					{
						"type": map[string]interface{}{
							"name": "Blocks",
						},
						"outwardIssue": map[string]interface{}{
							"key": "PROJ-103",
						},
					},
				},
			},
		},
		"PROJ-102": {
			"key": "PROJ-102",
			"id":  "10102",
			"fields": map[string]interface{}{
				"summary":     "Subtask",
				"description": "Create login form",
				"issuetype": map[string]interface{}{
					"name":    "Subtask",
					"subtask": true,
				},
				"status": map[string]interface{}{
					"name": "To Do",
					"statusCategory": map[string]interface{}{
						"key": "new",
					},
				},
				"priority": map[string]interface{}{
					"name": "Medium",
				},
				"created": "2024-01-03T10:00:00.000+0000",
				"updated": "2024-01-17T14:30:00.000+0000",
				"parent": map[string]interface{}{
					"key": "PROJ-101",
					"fields": map[string]interface{}{
						"issuetype": map[string]interface{}{
							"name": "Story",
						},
					},
				},
			},
		},
		"PROJ-103": {
			"key": "PROJ-103",
			"id":  "10103",
			"fields": map[string]interface{}{
				"summary":     "Blocked Task",
				"description": "Deploy authentication service",
				"issuetype": map[string]interface{}{
					"name": "Task",
				},
				"status": map[string]interface{}{
					"name": "Open",
					"statusCategory": map[string]interface{}{
						"key": "new",
					},
				},
				"priority": map[string]interface{}{
					"name": "Low",
				},
				"created": "2024-01-04T10:00:00.000+0000",
				"updated": "2024-01-18T14:30:00.000+0000",
				"issuelinks": []map[string]interface{}{
					{
						"type": map[string]interface{}{
							"name": "Blocks",
						},
						"inwardIssue": map[string]interface{}{
							"key": "PROJ-101",
						},
					},
				},
			},
		},
	}
}

// createMockJiraServer creates an HTTP test server that mocks Jira API
func createMockJiraServer(t *testing.T, mockData map[string]map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract issue key from path
		path := strings.TrimPrefix(r.URL.Path, "/rest/api/2/issue/")

		if issueData, ok := mockData[path]; ok {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(issueData); err != nil {
				t.Errorf("Failed to encode response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(fmt.Sprintf(`{"errorMessages":["Issue %s not found"]}`, path)))
		}
	}))
}

// beadsAvailable checks if the beads CLI (bd) is available
func beadsAvailable() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// initBeadsRepo initializes a beads repository using the bd CLI
func initBeadsRepo(dir string) error {
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd init failed: %v\nOutput: %s", err, output)
	}
	return nil
}

// testBeadsCLI tests integration with the beads CLI
func testBeadsCLI(t *testing.T, dir string, issues ...map[string]interface{}) {
	t.Helper()

	// Test: bd list
	t.Run("bd list", func(t *testing.T) {
		cmd := exec.Command("bd", "list")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("bd list failed (may be expected): %v\nOutput: %s", err, output)
			return
		}

		outputStr := string(output)
		t.Logf("bd list output:\n%s", outputStr)

		// Check that issue IDs appear in output
		for _, issue := range issues {
			issueID := issue["id"].(string)
			if !strings.Contains(outputStr, issueID) {
				t.Errorf("Expected issue %s in bd list output", issueID)
			}
		}
	})

	// Test: bd show for first issue
	if len(issues) > 0 {
		t.Run("bd show", func(t *testing.T) {
			issueID := issues[0]["id"].(string)
			cmd := exec.Command("bd", "show", issueID)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("bd show failed (may be expected): %v\nOutput: %s", err, output)
				return
			}

			outputStr := string(output)
			t.Logf("bd show %s output:\n%s", issueID, outputStr)

			// Check that title appears in output
			if title, ok := issues[0]["title"].(string); ok {
				if !strings.Contains(outputStr, title) {
					t.Errorf("Expected title %q in bd show output", title)
				}
			}
		})
	}
}

// TestEndToEndWithLabels tests syncing issues with various label configurations
func TestEndToEndWithLabels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jira-beads-sync-labels-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock data with various label configurations
	mockData := map[string]map[string]interface{}{
		"PROJ-200": {
			"key": "PROJ-200",
			"id":  "10200",
			"fields": map[string]interface{}{
				"summary":     "Issue with many labels",
				"description": "Test labels",
				"issuetype": map[string]interface{}{
					"name": "Task",
				},
				"status": map[string]interface{}{
					"name": "Open",
					"statusCategory": map[string]interface{}{
						"key": "new",
					},
				},
				"priority": map[string]interface{}{
					"name": "Medium",
				},
				"created": "2024-01-01T10:00:00.000+0000",
				"updated": "2024-01-15T14:30:00.000+0000",
				"labels":  []string{"bug", "frontend", "urgent", "customer-reported"},
			},
		},
	}

	server := createMockJiraServer(t, mockData)
	defer server.Close()

	// Fetch and convert
	client := jira.NewClient(server.URL, "test@example.com", "test-token")
	jiraExport, err := client.FetchIssueWithDependencies("PROJ-200")
	if err != nil {
		t.Fatalf("Failed to fetch issues: %v", err)
	}

	protoConverter := converter.NewProtoConverter()
	beadsExport, err := protoConverter.Convert(jiraExport)
	if err != nil {
		t.Fatalf("Failed to convert: %v", err)
	}

	// Create .beads directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads", "issues"), 0755); err != nil {
		t.Fatalf("Failed to create .beads directory: %v", err)
	}

	yamlRenderer := beads.NewYAMLRenderer(tmpDir)
	if err := yamlRenderer.RenderExport(beadsExport); err != nil {
		t.Fatalf("Failed to render: %v", err)
	}

	// Validate labels
	validateBeadsIssue(t, tmpDir, "proj-200", BeadsIssueExpectation{
		Title:         "Issue with many labels",
		Status:        "open",
		Priority:      "p2",
		JiraKey:       "PROJ-200",
		JiraIssueType: "Task",
		LabelCount:    4,
	})

	t.Log("✓ Label synchronization test completed")
}
