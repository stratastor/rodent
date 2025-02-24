// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ad_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stratastor/rodent/pkg/ad"
)

// randomString generates a unique string based on the current timestamp.
func randomString(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// TestUserLifecycle tests user CRUD operations with additional attributes
func TestUserLifecycle(t *testing.T) {
	client, err := ad.New()
	if err != nil {
		t.Fatalf("Failed to create ADClient: %v", err)
	}
	defer client.Close()

	username := randomString("JacksColdSweat")
	user := &ad.User{
		CN:             username,
		SAMAccountName: username,
		Password:       "SpaceMonkey#42!",
		GivenName:      "Robert",
		Surname:        "Paulson",
		Description:    "In Tyler we trust",
		DisplayName:    "Robert Paulson",
		Title:          "Project Mayhem Coordinator",
		Department:     "Soap Manufacturing",
		Company:        "Paper Street Soap Co.",
		PhoneNumber:    "555-0134",
		Mobile:         "555-0135",
		Mail:           "robert.paulson@paperstreet.com",
		EmployeeID:     "PM-001",
	}

	// Create and verify user attributes
	if err := client.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	entries, err := client.SearchUser(username)
	if err != nil {
		t.Fatalf("SearchUser failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("User %s not found after creation", username)
	}

	// Verify all attributes
	entry := entries[0]
	verifyAttributes := map[string]string{
		"displayName":     user.DisplayName,
		"title":           user.Title,
		"department":      user.Department,
		"company":         user.Company,
		"telephoneNumber": user.PhoneNumber,
		"mobile":          user.Mobile,
		"mail":            user.Mail,
		"employeeID":      user.EmployeeID,
	}

	for attr, expected := range verifyAttributes {
		if got := entry.GetAttributeValue(attr); got != expected {
			t.Errorf("Expected %s='%s', got '%s'", attr, expected, got)
		}
	}

	// Update multiple attributes
	user.Title = "Fight Club Organizer"
	user.Department = "Underground Operations"
	user.PhoneNumber = "555-0136"
	user.Description = "His name was Robert Paulson"

	if err := client.UpdateUser(user); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	// Verify updates
	entries, err = client.SearchUser(username)
	if err != nil {
		t.Fatalf("SearchUser after update failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("User %s not found after update", username)
	}

	entry = entries[0]
	updatedAttrs := map[string]string{
		"title":           "Fight Club Organizer",
		"department":      "Underground Operations",
		"telephoneNumber": "555-0136",
		"description":     "His name was Robert Paulson",
	}

	for attr, expected := range updatedAttrs {
		if got := entry.GetAttributeValue(attr); got != expected {
			t.Errorf("After update: Expected %s='%s', got '%s'", attr, expected, got)
		}
	}

	// Delete and verify
	if err := client.DeleteUser(username); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	entries, err = client.SearchUser(username)
	if err != nil {
		t.Fatalf("SearchUser after deletion failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("User %s still exists after deletion", username)
	}
}

// TestGroupLifecycle creates a group, verifies its creation, updates its attributes, and then deletes it.
func TestGroupLifecycle(t *testing.T) {
	client, err := ad.New()
	if err != nil {
		t.Fatalf("Failed to create ADClient: %v", err)
	}
	defer client.Close()

	groupName := randomString("BlowUp")
	group := &ad.Group{
		CN:             groupName,
		SAMAccountName: groupName,
		Description:    "Project Mayhem Operations",
		DisplayName:    "Project Mayhem Team",
		Mail:           "project.mayhem@paperstreet.com",
		GroupType:      4, // Security group
		Scope:          "Global",
		Managed:        true,
	}

	if err := client.CreateGroup(group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	entries, err := client.SearchGroup(groupName)
	if err != nil {
		t.Fatalf("SearchGroup failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("Group %s not found after creation", groupName)
	}

	// Verify attributes
	entry := entries[0]
	verifyAttributes := map[string]string{
		"displayName": group.DisplayName,
		"mail":        group.Mail,
		"groupType":   "4",
	}

	for attr, expected := range verifyAttributes {
		if got := entry.GetAttributeValue(attr); got != expected {
			t.Errorf("Expected %s='%s', got '%s'", attr, expected, got)
		}
	}

	// Test update with new values
	group.DisplayName = "Space Monkeys"
	group.Description = "The First Rule of Project Mayhem"
	group.Mail = "space.monkeys@paperstreet.com"

	if err := client.UpdateGroup(group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	// Verify updates
	entries, err = client.SearchGroup(groupName)
	if err != nil {
		t.Fatalf("SearchGroup after update failed: %v", err)
	}

	entry = entries[0]
	updatedAttrs := map[string]string{
		"displayName": "Space Monkeys",
		"description": "The First Rule of Project Mayhem",
		"mail":        "space.monkeys@paperstreet.com",
	}

	for attr, expected := range updatedAttrs {
		if got := entry.GetAttributeValue(attr); got != expected {
			t.Errorf("After update: Expected %s='%s', got '%s'", attr, expected, got)
		}
	}

	// Delete and verify
	if err := client.DeleteGroup(groupName); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
}

// TestComputerLifecycle creates a computer object, verifies its creation, updates its attributes, and then deletes it.
func TestComputerLifecycle(t *testing.T) {
	client, err := ad.New()
	if err != nil {
		t.Fatalf("Failed to create ADClient: %v", err)
	}
	defer client.Close()

	compName := randomString("SoapBox")
	comp := &ad.Computer{
		CN:             compName,
		SAMAccountName: compName + "$", // Typically computer accounts end with a '$'
		Description:    "Paper Street Soap Company",
	}

	// Create the computer object.
	if err := client.CreateComputer(comp); err != nil {
		t.Fatalf("CreateComputer failed: %v", err)
	}

	// Search for the computer.
	entries, err := client.SearchComputer(comp.SAMAccountName)
	if err != nil {
		t.Fatalf("SearchComputer failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("Computer %s not found after creation", compName)
	}

	// Update the computer's description.
	comp.Description = "Updated computer description"
	if err := client.UpdateComputer(comp); err != nil {
		t.Fatalf("UpdateComputer failed: %v", err)
	}

	// Verify the update.
	entries, err = client.SearchComputer(comp.SAMAccountName)
	if err != nil {
		t.Fatalf("SearchComputer after update failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("Computer %s not found after update", comp.SAMAccountName)
	}
	updatedDesc := entries[0].GetAttributeValue("description")
	if updatedDesc != "Updated computer description" {
		t.Errorf(
			"Expected computer description 'Updated computer description', got '%s'",
			updatedDesc,
		)
	}

	// Delete the computer.
	if err := client.DeleteComputer(compName); err != nil {
		t.Fatalf("DeleteComputer failed: %v", err)
	}

	// Verify deletion.
	entries, err = client.SearchComputer(comp.SAMAccountName)
	if err != nil {
		t.Fatalf("SearchComputer after deletion failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Computer %s still exists after deletion", compName)
	}
}
