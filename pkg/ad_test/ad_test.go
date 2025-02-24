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

// TestUserLifecycle creates a user, verifies its creation, updates its attributes, and then deletes it.
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
	}

	// Create the user.
	if err := client.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Search for the user.
	entries, err := client.SearchUser(username)
	if err != nil {
		t.Fatalf("SearchUser failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("User %s not found after creation", username)
	}
	t.Logf("User created with DN: %s", entries[0].DN)

	// Update the user's description.
	user.Description = "Updated description"
	if err := client.UpdateUser(user); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	// Verify the update.
	entries, err = client.SearchUser(username)
	if err != nil {
		t.Fatalf("SearchUser after update failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("User %s not found after update", username)
	}
	updatedDesc := entries[0].GetAttributeValue("description")
	if updatedDesc != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", updatedDesc)
	}

	// Delete the user.
	if err := client.DeleteUser(username); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Verify deletion.
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
		Description:    "Destroying aspects of modern society associated with consumerism",
	}

	// Create the group.
	if err := client.CreateGroup(group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// Search for the group.
	entries, err := client.SearchGroup(groupName)
	if err != nil {
		t.Fatalf("SearchGroup failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("Group %s not found after creation", groupName)
	}
	t.Logf("Group created with DN: %s", entries[0].DN)

	// Update the group's description.
	group.Description = "Updated group description"
	if err := client.UpdateGroup(group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	// Verify the update.
	entries, err = client.SearchGroup(groupName)
	if err != nil {
		t.Fatalf("SearchGroup after update failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("Group %s not found after update", groupName)
	}
	updatedDesc := entries[0].GetAttributeValue("description")
	if updatedDesc != "Updated group description" {
		t.Errorf("Expected group description 'Updated group description', got '%s'", updatedDesc)
	}

	// Delete the group.
	if err := client.DeleteGroup(groupName); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}

	// Verify deletion.
	entries, err = client.SearchGroup(groupName)
	if err != nil {
		t.Fatalf("SearchGroup after deletion failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Group %s still exists after deletion", groupName)
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
	t.Logf("Computer created with DN: %s", entries[0].DN)

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
