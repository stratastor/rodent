// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/stratastor/logger"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/pkg/errors"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

const (
	// MinUserUID is the minimum UID for regular users (system users have UID < 1000)
	MinUserUID = 1000
	// MinGroupGID is the minimum GID for regular groups (system groups have GID < 1000)
	MinGroupGID = 1000
)

// Protected system users that cannot be deleted
var protectedUsers = []string{
	"root",
	"daemon",
	"bin",
	"sys",
	"sync",
	"games",
	"man",
	"lp",
	"mail",
	"news",
	"uucp",
	"proxy",
	"www-data",
	"backup",
	"list",
	"irc",
	"gnats",
	"nobody",
	"systemd-network",
	"systemd-resolve",
	"messagebus",
	"systemd-timesync",
	"sshd",
	"ubuntu",
	"rodent",
	"strata",
}

// Protected system groups that cannot be deleted
var protectedGroups = []string{
	"root",
	"daemon",
	"bin",
	"sys",
	"adm",
	"tty",
	"disk",
	"lp",
	"mail",
	"news",
	"uucp",
	"man",
	"proxy",
	"kmem",
	"dialout",
	"fax",
	"voice",
	"cdrom",
	"floppy",
	"tape",
	"sudo",
	"audio",
	"dip",
	"www-data",
	"backup",
	"operator",
	"list",
	"irc",
	"src",
	"gnats",
	"shadow",
	"utmp",
	"video",
	"sasl",
	"plugdev",
	"staff",
	"games",
	"users",
	"nogroup",
	"ubuntu",
	"rodent",
	"strata",
}

// UserManager manages local system users and groups
type UserManager struct {
	executor CommandExecutor
	logger   logger.Logger
}

// NewUserManager creates a new user manager
func NewUserManager(logger logger.Logger) *UserManager {
	return &UserManager{
		executor: &commandExecutorWrapper{
			executor: generalCmd.NewCommandExecutor(true), // Use sudo for user operations
		},
		logger: logger,
	}
}

// GetUsers lists regular system users (UID >= 1000, excludes system users)
func (um *UserManager) GetUsers(ctx context.Context) ([]User, error) {
	users := []User{}

	// Parse /etc/passwd
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, errors.Wrap(err, errors.SystemInfoCollectionFailed)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		user, err := um.parsePasswdLine(line)
		if err != nil {
			um.logger.Warn("Failed to parse passwd line", "line", line, "error", err)
			continue
		}

		// Skip system users (UID < 1000) to improve performance
		if user.UID < MinUserUID {
			continue
		}

		// Get additional user information only for regular users
		um.enrichUserInfo(ctx, user)
		users = append(users, *user)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.New(
			errors.ServerInternalError,
			"Error reading /etc/passwd: "+err.Error(),
		)
	}

	return users, nil
}

// GetUser gets a specific user by username (optimized to avoid full user list scan)
func (um *UserManager) GetUser(ctx context.Context, username string) (*User, error) {
	if username == "" {
		return nil, errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Direct lookup from /etc/passwd instead of scanning all users
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, errors.Wrap(err, errors.SystemInfoCollectionFailed)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		user, err := um.parsePasswdLine(line)
		if err != nil {
			continue
		}

		// Found the user
		if user.Username == username {
			// Only enrich user info if it's a regular user (UID >= 1000)
			// System users don't need enrichment and this improves performance
			if user.UID >= MinUserUID {
				um.enrichUserInfo(ctx, user)
			}
			return user, nil
		}
	}

	return nil, errors.New(
		errors.ServerRequestValidation,
		fmt.Sprintf("User '%s' not found", username),
	)
}

// CreateUser creates a new system user
func (um *UserManager) CreateUser(ctx context.Context, request CreateUserRequest) error {
	// Validate the request
	if err := um.validateCreateUserRequest(request); err != nil {
		return err
	}

	um.logger.Info(
		"Creating system user",
		"username",
		request.Username,
		"system",
		request.SystemUser,
	)

	// Build useradd command
	args := []string{}

	if request.SystemUser {
		args = append(args, "--system")
	}

	// Add UID if explicitly specified
	if request.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*request.UID))
	}

	if request.FullName != "" {
		args = append(args, "--comment", request.FullName)
	}

	if request.HomeDir != "" {
		args = append(args, "--home-dir", request.HomeDir)
	}

	if request.Shell != "" {
		args = append(args, "--shell", request.Shell)
	}

	if request.CreateHome {
		args = append(args, "--create-home")
	} else {
		args = append(args, "--no-create-home")
	}

	// Add encrypted password if provided
	if request.Password != "" {
		encryptedPassword, err := um.encryptPassword(ctx, request.Password)
		if err != nil {
			um.logger.Error("Failed to encrypt password", "username", request.Username, "error", err)
			return errors.Wrap(err, errors.SystemUserPasswordEncryptFailed).
				WithMetadata("username", request.Username)
		}
		// Note: ExecuteCommand passes args directly without shell interpretation,
		// so $ characters in the encrypted password should be preserved correctly
		args = append(args, "--password", encryptedPassword)
		um.logger.Debug("Setting encrypted password for user", "username", request.Username, "hash_length", len(encryptedPassword))
	}

	// Add username as final argument
	args = append(args, request.Username)

	// Execute useradd command
	result, err := um.executor.ExecuteCommand(ctx, "useradd", args...)
	if err != nil {
		um.logger.Error("Failed to create user", "username", request.Username, "error", err)
		return errors.Wrap(err, errors.SystemUserCreateFailed).
			WithMetadata("username", request.Username).
			WithMetadata("output", result.Stdout)
	}

	// Add user to additional groups if specified
	if len(request.Groups) > 0 {
		for _, group := range request.Groups {
			_, err = um.executor.ExecuteCommand(ctx, "usermod", "-a", "-G", group, request.Username)
			if err != nil {
				um.logger.Warn(
					"Failed to add user to group",
					"username",
					request.Username,
					"group",
					group,
					"error",
					err,
				)
				// Don't fail the entire operation for group membership failures
			}
		}
	}

	// Password is now set during user creation using useradd --password
	// No need for separate password setting step

	um.logger.Info("Successfully created system user", "username", request.Username)

	// Add user to Samba database for SMB/CIFS authentication
	// This is done regardless of security mode to ensure seamless mode transitions
	if request.Password != "" {
		if err := um.addUserToSamba(ctx, request.Username, request.Password); err != nil {
			um.logger.Warn(
				"Failed to add user to Samba database (SMB access may not work)",
				"username", request.Username,
				"error", err,
			)
			// Don't fail the entire operation - user was created successfully in the system
			// SMB password can be set later if needed
		} else {
			um.logger.Debug("User synchronized to Samba database", "username", request.Username)
		}
	}

	// Emit user creation event with structured payload
	userPayload := &eventspb.SystemUserPayload{
		Username:    request.Username,
		DisplayName: request.FullName,
		Groups:      request.Groups,
		Operation:   eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_CREATED,
	}

	userMeta := map[string]string{
		"component": "system-user-manager",
		"action":    "create",
		"user":      request.Username,
		"shell":     request.Shell,
		"home_dir":  request.HomeDir,
	}

	events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_INFO, userPayload, userMeta)
	
	return nil
}

// DeleteUser deletes a system user
func (um *UserManager) DeleteUser(ctx context.Context, username string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Safety check: prevent deletion of protected users
	if um.isProtectedUser(username) {
		return errors.New(errors.SystemUserProtected,
			fmt.Sprintf("Cannot delete protected system user '%s'", username))
	}

	// Check if user exists
	_, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	um.logger.Info("Deleting system user", "username", username)

	// Remove user from Samba database first
	// Done before userdel to ensure cleanup even if system deletion fails
	if err := um.removeUserFromSamba(ctx, username); err != nil {
		um.logger.Warn(
			"Failed to remove user from Samba database",
			"username", username,
			"error", err,
		)
		// Continue with system user deletion
	}

	// Delete user with home directory
	result, err := um.executor.ExecuteCommand(ctx, "userdel", "-r", username)
	if err != nil {
		um.logger.Error("Failed to delete user", "username", username, "error", err)
		return errors.Wrap(err, errors.SystemUserDeleteFailed).
			WithMetadata("username", username).
			WithMetadata("output", result.Stdout)
	}

	um.logger.Info("Successfully deleted system user", "username", username)
	
	// Emit user deletion event with structured payload
	userPayload := &eventspb.SystemUserPayload{
		Username:  username,
		Operation: eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_DELETED,
	}

	userMeta := map[string]string{
		"component": "system-user-manager",
		"action":    "delete",
		"user":      username,
	}

	events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_INFO, userPayload, userMeta)
	
	return nil
}

// GetGroups lists regular system groups (GID >= 1000, excludes system groups)
func (um *UserManager) GetGroups(ctx context.Context) ([]Group, error) {
	groups := []Group{}

	// Parse /etc/group
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, errors.New(
			errors.ServerInternalError,
			"Failed to read /etc/group: "+err.Error(),
		)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		group, err := um.parseGroupLine(line)
		if err != nil {
			um.logger.Warn("Failed to parse group line", "line", line, "error", err)
			continue
		}

		// Skip system groups (GID < 1000) to improve performance
		if group.GID < MinGroupGID {
			continue
		}

		groups = append(groups, *group)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, errors.SystemInfoCollectionFailed)
	}

	return groups, nil
}

// GetGroup gets a specific group by name (optimized to avoid full group list scan)
func (um *UserManager) GetGroup(ctx context.Context, groupName string) (*Group, error) {
	if groupName == "" {
		return nil, errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Direct lookup from /etc/group instead of scanning all groups
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, errors.New(
			errors.ServerInternalError,
			"Failed to read /etc/group: "+err.Error(),
		)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		group, err := um.parseGroupLine(line)
		if err != nil {
			continue
		}

		// Found the group
		if group.Name == groupName {
			return group, nil
		}
	}

	return nil, errors.New(
		errors.ServerRequestValidation,
		fmt.Sprintf("Group '%s' not found", groupName),
	)
}

// CreateGroup creates a new system group
func (um *UserManager) CreateGroup(ctx context.Context, request CreateGroupRequest) error {
	if err := um.validateCreateGroupRequest(request); err != nil {
		return err
	}

	um.logger.Info("Creating system group", "name", request.Name, "system", request.SystemGroup)

	// Build groupadd command
	args := []string{}

	if request.SystemGroup {
		args = append(args, "--system")
	}

	// Add GID if explicitly specified
	if request.GID != nil {
		args = append(args, "--gid", strconv.Itoa(*request.GID))
	}

	// Add group name as final argument
	args = append(args, request.Name)

	// Execute groupadd command
	result, err := um.executor.ExecuteCommand(ctx, "groupadd", args...)
	if err != nil {
		um.logger.Error("Failed to create group", "name", request.Name, "error", err)
		return errors.Wrap(err, errors.SystemGroupCreateFailed).
			WithMetadata("group", request.Name).
			WithMetadata("output", result.Stdout)
	}

	um.logger.Info("Successfully created system group", "name", request.Name)
	return nil
}

// DeleteGroup deletes a system group
func (um *UserManager) DeleteGroup(ctx context.Context, groupName string) error {
	if groupName == "" {
		return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Safety check: prevent deletion of protected groups
	if um.isProtectedGroup(groupName) {
		return errors.New(errors.SystemGroupProtected,
			fmt.Sprintf("Cannot delete protected system group '%s'", groupName))
	}

	// Check if group exists
	group, err := um.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}

	// Safety check: prevent deletion of groups with members
	if len(group.Members) > 0 {
		return errors.New(errors.SystemGroupMembershipFailed,
			fmt.Sprintf("Cannot delete group '%s' - it has %d members. Remove members first.",
				groupName, len(group.Members)))
	}

	um.logger.Info("Deleting system group", "name", groupName)

	// Execute groupdel command
	result, err := um.executor.ExecuteCommand(ctx, "groupdel", groupName)
	if err != nil {
		um.logger.Error("Failed to delete group", "name", groupName, "error", err)
		return errors.Wrap(err, errors.SystemGroupDeleteFailed).
			WithMetadata("group", groupName).
			WithMetadata("output", result.Stdout)
	}

	um.logger.Info("Successfully deleted system group", "name", groupName)
	return nil
}

// UpdateUser updates an existing system user
func (um *UserManager) UpdateUser(ctx context.Context, request UpdateUserRequest) error {
	// Validate username
	if request.Username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Check if user is protected
	if um.isProtectedUser(request.Username) {
		return errors.New(errors.SystemUserProtected,
			fmt.Sprintf("Cannot modify protected system user '%s'", request.Username))
	}

	// Check if user exists
	_, err := um.GetUser(ctx, request.Username)
	if err != nil {
		return err
	}

	um.logger.Info("Updating system user", "username", request.Username)

	// Build usermod command args based on what's being updated
	hasChanges := false

	if request.FullName != nil {
		_, err = um.executor.ExecuteCommand(ctx, "usermod", "--comment", *request.FullName, request.Username)
		if err != nil {
			um.logger.Error("Failed to update user full name", "username", request.Username, "error", err)
			return errors.Wrap(err, errors.SystemUserModifyFailed).
				WithMetadata("username", request.Username).
				WithMetadata("field", "full_name")
		}
		hasChanges = true
	}

	if request.Shell != nil {
		// Validate shell
		validShells := []string{
			"/bin/bash", "/bin/sh", "/bin/dash", "/bin/zsh",
			"/usr/bin/fish", "/bin/false", "/sbin/nologin",
		}
		valid := false
		for _, shell := range validShells {
			if *request.Shell == shell {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New(errors.SystemUserInvalidShell, "Invalid shell specified")
		}

		_, err = um.executor.ExecuteCommand(ctx, "usermod", "--shell", *request.Shell, request.Username)
		if err != nil {
			um.logger.Error("Failed to update user shell", "username", request.Username, "error", err)
			return errors.Wrap(err, errors.SystemUserModifyFailed).
				WithMetadata("username", request.Username).
				WithMetadata("field", "shell")
		}
		hasChanges = true
	}

	if request.HomeDir != nil {
		_, err = um.executor.ExecuteCommand(ctx, "usermod", "--home", *request.HomeDir, request.Username)
		if err != nil {
			um.logger.Error("Failed to update user home directory", "username", request.Username, "error", err)
			return errors.Wrap(err, errors.SystemUserModifyFailed).
				WithMetadata("username", request.Username).
				WithMetadata("field", "home_dir")
		}
		hasChanges = true
	}

	if request.UID != nil {
		_, err = um.executor.ExecuteCommand(ctx, "usermod", "--uid", strconv.Itoa(*request.UID), request.Username)
		if err != nil {
			um.logger.Error("Failed to update user UID", "username", request.Username, "error", err)
			return errors.Wrap(err, errors.SystemUserModifyFailed).
				WithMetadata("username", request.Username).
				WithMetadata("field", "uid")
		}
		hasChanges = true
	}

	if !hasChanges {
		return errors.New(errors.ServerRequestValidation, "No fields to update")
	}

	um.logger.Info("Successfully updated system user", "username", request.Username)

	// Emit user update event
	userPayload := &eventspb.SystemUserPayload{
		Username:  request.Username,
		Operation: eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_MODIFIED,
	}
	if request.FullName != nil {
		userPayload.DisplayName = *request.FullName
	}

	userMeta := map[string]string{
		"component": "system-user-manager",
		"action":    "update",
		"user":      request.Username,
	}

	events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_INFO, userPayload, userMeta)

	return nil
}

// SetPassword sets or changes a user's password
func (um *UserManager) SetPassword(ctx context.Context, username, password string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	if password == "" {
		return errors.New(errors.ServerRequestValidation, "Password cannot be empty")
	}

	// Check if user exists
	_, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	um.logger.Info("Setting password for user", "username", username)

	// Encrypt the password
	encryptedPassword, err := um.encryptPassword(ctx, password)
	if err != nil {
		um.logger.Error("Failed to encrypt password", "username", username, "error", err)
		return errors.Wrap(err, errors.SystemUserPasswordEncryptFailed).
			WithMetadata("username", username)
	}

	// Use usermod to set password
	_, err = um.executor.ExecuteCommand(ctx, "usermod", "--password", encryptedPassword, username)
	if err != nil {
		um.logger.Error("Failed to set password", "username", username, "error", err)
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("field", "password")
	}

	// Update Samba password for SMB/CIFS authentication
	// If PAM smbpass is configured, usermod should have synced automatically,
	// but we do explicit sync for robustness
	if err := um.updateSambaPassword(ctx, username, password); err != nil {
		um.logger.Warn(
			"Failed to update Samba password (SMB access may require manual sync)",
			"username", username,
			"error", err,
		)
		// Don't fail the entire operation - system password was set successfully
	} else {
		um.logger.Debug("Synchronized password to Samba database", "username", username)
	}

	um.logger.Info("Successfully set password for user", "username", username)
	return nil
}

// LockUser locks a user account
func (um *UserManager) LockUser(ctx context.Context, username string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Check if user is protected
	if um.isProtectedUser(username) {
		return errors.New(errors.SystemUserProtected,
			fmt.Sprintf("Cannot lock protected system user '%s'", username))
	}

	// Check if user exists
	user, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	if user.Locked {
		um.logger.Debug("User already locked", "username", username)
		return nil // Already locked, no error
	}

	um.logger.Info("Locking user account", "username", username)

	_, err = um.executor.ExecuteCommand(ctx, "usermod", "--lock", username)
	if err != nil {
		um.logger.Error("Failed to lock user", "username", username, "error", err)
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("operation", "lock")
	}

	um.logger.Info("Successfully locked user account", "username", username)

	// Emit user lock event
	userPayload := &eventspb.SystemUserPayload{
		Username:  username,
		Operation: eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_LOCKED,
	}

	userMeta := map[string]string{
		"component": "system-user-manager",
		"action":    "lock",
		"user":      username,
	}

	events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_WARN, userPayload, userMeta)

	return nil
}

// UnlockUser unlocks a user account
func (um *UserManager) UnlockUser(ctx context.Context, username string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Check if user exists
	user, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	if !user.Locked {
		um.logger.Debug("User already unlocked", "username", username)
		return nil // Already unlocked, no error
	}

	um.logger.Info("Unlocking user account", "username", username)

	_, err = um.executor.ExecuteCommand(ctx, "usermod", "--unlock", username)
	if err != nil {
		um.logger.Error("Failed to unlock user", "username", username, "error", err)
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("operation", "unlock")
	}

	um.logger.Info("Successfully unlocked user account", "username", username)

	// Emit user unlock event
	userPayload := &eventspb.SystemUserPayload{
		Username:  username,
		Operation: eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_UNLOCKED,
	}

	userMeta := map[string]string{
		"component": "system-user-manager",
		"action":    "unlock",
		"user":      username,
	}

	events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_INFO, userPayload, userMeta)

	return nil
}

// AddUserToGroup adds a user to a group
func (um *UserManager) AddUserToGroup(ctx context.Context, username, groupName string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	if groupName == "" {
		return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Check if user exists
	_, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	// Check if group exists
	_, err = um.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}

	um.logger.Info("Adding user to group", "username", username, "group", groupName)

	// Use usermod -a -G to append the group (preserves existing groups)
	_, err = um.executor.ExecuteCommand(ctx, "usermod", "-a", "-G", groupName, username)
	if err != nil {
		um.logger.Error("Failed to add user to group", "username", username, "group", groupName, "error", err)
		return errors.Wrap(err, errors.SystemGroupMembershipFailed).
			WithMetadata("username", username).
			WithMetadata("group", groupName)
	}

	um.logger.Info("Successfully added user to group", "username", username, "group", groupName)
	return nil
}

// RemoveUserFromGroup removes a user from a group
func (um *UserManager) RemoveUserFromGroup(ctx context.Context, username, groupName string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	if groupName == "" {
		return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Check if user exists
	user, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	// Check if group exists
	group, err := um.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}

	// Check if user is in the group
	isMember := false
	for _, member := range group.Members {
		if member == username {
			isMember = true
			break
		}
	}

	// Also check user's groups list
	if !isMember && user.Groups != nil {
		for _, grp := range user.Groups {
			if grp == groupName {
				isMember = true
				break
			}
		}
	}

	if !isMember {
		um.logger.Debug("User is not a member of the group", "username", username, "group", groupName)
		return nil // Not a member, no error
	}

	um.logger.Info("Removing user from group", "username", username, "group", groupName)

	// Use gpasswd -d to remove user from group
	_, err = um.executor.ExecuteCommand(ctx, "gpasswd", "-d", username, groupName)
	if err != nil {
		um.logger.Error("Failed to remove user from group", "username", username, "group", groupName, "error", err)
		return errors.Wrap(err, errors.SystemGroupMembershipFailed).
			WithMetadata("username", username).
			WithMetadata("group", groupName)
	}

	um.logger.Info("Successfully removed user from group", "username", username, "group", groupName)
	return nil
}

// SetPrimaryGroup sets a user's primary group
func (um *UserManager) SetPrimaryGroup(ctx context.Context, username, groupName string) error {
	if username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	if groupName == "" {
		return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Check if user exists
	_, err := um.GetUser(ctx, username)
	if err != nil {
		return err
	}

	// Check if group exists
	_, err = um.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}

	um.logger.Info("Setting primary group for user", "username", username, "group", groupName)

	// Use usermod -g to set primary group
	_, err = um.executor.ExecuteCommand(ctx, "usermod", "-g", groupName, username)
	if err != nil {
		um.logger.Error("Failed to set primary group", "username", username, "group", groupName, "error", err)
		return errors.Wrap(err, errors.SystemGroupMembershipFailed).
			WithMetadata("username", username).
			WithMetadata("group", groupName)
	}

	um.logger.Info("Successfully set primary group for user", "username", username, "group", groupName)
	return nil
}

// GetUserGroups gets all groups a user belongs to
func (um *UserManager) GetUserGroups(ctx context.Context, username string) ([]string, error) {
	if username == "" {
		return nil, errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Check if user exists and get groups
	user, err := um.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	return user.Groups, nil
}

// parsePasswdLine parses a line from /etc/passwd
func (um *UserManager) parsePasswdLine(line string) (*User, error) {
	fields := strings.Split(line, ":")
	if len(fields) != 7 {
		return nil, fmt.Errorf("invalid passwd line format")
	}

	uid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("invalid UID: %s", fields[2])
	}

	gid, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, fmt.Errorf("invalid GID: %s", fields[3])
	}

	user := &User{
		Username: fields[0],
		UID:      uid,
		GID:      gid,
		FullName: fields[4],
		HomeDir:  fields[5],
		Shell:    fields[6],
	}

	return user, nil
}

// parseGroupLine parses a line from /etc/group
func (um *UserManager) parseGroupLine(line string) (*Group, error) {
	fields := strings.Split(line, ":")
	if len(fields) != 4 {
		return nil, fmt.Errorf("invalid group line format")
	}

	gid, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("invalid GID: %s", fields[2])
	}

	members := []string{}
	if fields[3] != "" {
		members = strings.Split(fields[3], ",")
	}

	group := &Group{
		Name:    fields[0],
		GID:     gid,
		Members: members,
	}

	return group, nil
}

// enrichUserInfo adds additional information to a user
func (um *UserManager) enrichUserInfo(ctx context.Context, user *User) {
	// Get user's groups
	result, err := um.executor.ExecuteCommand(ctx, "groups", user.Username)
	if err == nil {
		output := strings.TrimSpace(result.Stdout)
		// Output format: "username : group1 group2 group3"
		parts := strings.SplitN(output, ":", 2)
		if len(parts) == 2 {
			groupsStr := strings.TrimSpace(parts[1])
			if groupsStr != "" {
				user.Groups = strings.Fields(groupsStr)
			}
		}
	}

	// Check if user is locked
	result, err = um.executor.ExecuteCommand(ctx, "passwd", "-S", user.Username)
	if err == nil {
		// Output format: "username status ..."
		fields := strings.Fields(result.Stdout)
		if len(fields) >= 2 {
			status := fields[1]
			user.Locked = (status == "L" || status == "LK")
		}
	}

	// Get last login time using 'last' command
	um.setLastLoginTime(ctx, user)
}

// encryptPassword encrypts a password using openssl for use with useradd -p
func (um *UserManager) encryptPassword(ctx context.Context, password string) (string, error) {
	// Use openssl passwd to generate a secure encrypted password
	// -6 uses SHA-512 which is the modern standard
	result, err := um.executor.ExecuteCommand(ctx, "openssl", "passwd", "-6", password)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt password: %w", err)
	}
	
	// Return the encrypted password (trim whitespace)
	encryptedPassword := strings.TrimSpace(result.Stdout)
	if encryptedPassword == "" {
		return "", fmt.Errorf("openssl returned empty encrypted password")
	}
	
	return encryptedPassword, nil
}

// setLastLoginTime sets the last login time for a user using 'last' command
func (um *UserManager) setLastLoginTime(ctx context.Context, user *User) {
	// Use 'last' command with full time format and no hostname for consistent parsing
	result, err := um.executor.ExecuteCommand(ctx, "last", "--time-format", "full", "-R", "-n", "1", user.Username)
	if err != nil {
		// Don't fail if we can't get last login time, just log and continue
		um.logger.Debug("Failed to get last login time", "username", user.Username, "error", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	if len(lines) == 0 {
		return
	}

	// Find the first line that doesn't contain "wtmp begins"
	var loginLine string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "wtmp begins") {
			loginLine = line
			break
		}
	}

	if loginLine == "" {
		return
	}

	// Parse the last command output with --time-format full -R flags
	// Format examples:
	// "ubuntu   pts/0        Mon Aug 25 16:33:26 2025   still logged in"
	// "ubuntu   pts/1        Tue Aug  5 16:56:00 2025 - 17:55:00  (00:59)"
	um.parseLastLoginLine(loginLine, user)
}

// parseLastLoginLine parses a single line from 'last --time-format full -R' command output
func (um *UserManager) parseLastLoginLine(line string, user *User) {
	// Split by multiple spaces to separate fields
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return
	}

	// Skip username (fields[0]) and tty (fields[1])
	// Date/time starts from fields[2] and includes year
	// Format: "Mon Aug 25 16:33:26 2025"
	
	if len(fields) >= 6 {
		// Extract date/time components: weekday month day time year
		dateStr := strings.Join(fields[2:7], " ") // "Mon Aug 25 16:33:26 2025"
		
		if t, err := time.Parse("Mon Jan 2 15:04:05 2006", dateStr); err == nil {
			user.LastLogin = &t
			return
		}
		
		// Handle potential format variations (e.g., single digit day)
		if t, err := time.Parse("Mon Jan _2 15:04:05 2006", dateStr); err == nil {
			user.LastLogin = &t
			return
		}
	}
	
	um.logger.Debug("Failed to parse last login time", "username", user.Username, "line", line)
}

// isProtectedUser checks if a user is in the protected list
func (um *UserManager) isProtectedUser(username string) bool {
	for _, protected := range protectedUsers {
		if username == protected {
			return true
		}
	}
	return false
}

// isProtectedGroup checks if a group is in the protected list
func (um *UserManager) isProtectedGroup(groupName string) bool {
	for _, protected := range protectedGroups {
		if groupName == protected {
			return true
		}
	}
	return false
}

// validateCreateUserRequest validates a create user request
func (um *UserManager) validateCreateUserRequest(request CreateUserRequest) error {
	// Validate username
	if request.Username == "" {
		return errors.New(errors.SystemUserInvalidName, "Username cannot be empty")
	}

	// Username validation (POSIX compliant)
	usernameRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	if !usernameRegex.MatchString(request.Username) {
		return errors.New(
			errors.ServerRequestValidation,
			"Invalid username format (must be lowercase, start with letter/underscore, max 32 chars)",
		)
	}

	// Validate shell if provided
	if request.Shell != "" {
		validShells := []string{
			"/bin/bash",
			"/bin/sh",
			"/bin/dash",
			"/bin/zsh",
			"/usr/bin/fish",
			"/bin/false",
			"/sbin/nologin",
		}
		valid := false
		for _, shell := range validShells {
			if request.Shell == shell {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New(errors.SystemUserInvalidShell, "Invalid shell specified")
		}
	}

	// Validate groups if provided
	for _, group := range request.Groups {
		if group == "" {
			return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
		}
		groupRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
		if !groupRegex.MatchString(group) {
			return errors.New(
				errors.ServerRequestValidation,
				fmt.Sprintf("Invalid group name format: %s", group),
			)
		}
	}

	return nil
}

// validateCreateGroupRequest validates a create group request
func (um *UserManager) validateCreateGroupRequest(request CreateGroupRequest) error {
	if request.Name == "" {
		return errors.New(errors.SystemGroupInvalidName, "Group name cannot be empty")
	}

	// Group name validation (POSIX compliant)
	groupRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	if !groupRegex.MatchString(request.Name) {
		return errors.New(
			errors.ServerRequestValidation,
			"Invalid group name format (must be lowercase, start with letter/underscore, max 32 chars)",
		)
	}

	return nil
}

// Samba User Database Management
// These methods manage the Samba user database (passdb) to enable SMB/CIFS authentication.
// Users are synced to Samba database regardless of security mode (standalone/AD) to ensure
// seamless mode transitions and consistent SMB access for local users.

// addUserToSamba adds a user to the Samba password database
// This enables the user to authenticate to SMB/CIFS shares
func (um *UserManager) addUserToSamba(ctx context.Context, username, password string) error {
	// Use pdbedit to add user with password via stdin
	// -a = add user
	// -u username = specify username
	// -t = read password from stdin (requires password twice, one per line)
	cmd := exec.CommandContext(ctx, "sudo", "pdbedit", "-a", "-u", username, "-t")
	// pdbedit -t expects password twice for confirmation, each on a separate line
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("operation", "samba_add").
			WithMetadata("stdout", stdout.String()).
			WithMetadata("stderr", stderr.String())
	}

	um.logger.Debug("Added user to Samba database", "username", username)
	return nil
}

// removeUserFromSamba removes a user from the Samba password database
func (um *UserManager) removeUserFromSamba(ctx context.Context, username string) error {
	// Use pdbedit to remove user
	// -x = delete user
	// -u username = specify username
	result, err := um.executor.ExecuteCommand(ctx, "pdbedit", "-x", "-u", username)
	if err != nil {
		// Don't fail if user doesn't exist in Samba database
		if strings.Contains(result.Stdout, "not found") || strings.Contains(result.Stderr, "not found") {
			um.logger.Debug("User not found in Samba database (already removed)", "username", username)
			return nil
		}
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("operation", "samba_remove").
			WithMetadata("output", result.Stdout)
	}

	um.logger.Debug("Removed user from Samba database", "username", username)
	return nil
}

// updateSambaPassword updates a user's password in the Samba database
// Note: If PAM smbpass is configured, password changes via usermod will
// automatically sync. This method provides explicit sync for robustness.
func (um *UserManager) updateSambaPassword(ctx context.Context, username, password string) error {
	// Use pdbedit to update password
	// -a = add or update user (when user exists, this updates their password)
	// -u username = specify username
	// -t = read password from stdin (requires password twice, one per line)
	cmd := exec.CommandContext(ctx, "sudo", "pdbedit", "-a", "-u", username, "-t")
	// pdbedit -t expects password twice for confirmation, each on a separate line
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If user doesn't exist in Samba DB, add them
		output := stdout.String() + stderr.String()
		if strings.Contains(output, "not found") {
			um.logger.Debug("User not found in Samba database, adding", "username", username)
			return um.addUserToSamba(ctx, username, password)
		}
		return errors.Wrap(err, errors.SystemUserModifyFailed).
			WithMetadata("username", username).
			WithMetadata("operation", "samba_update_password").
			WithMetadata("stdout", stdout.String()).
			WithMetadata("stderr", stderr.String())
	}

	um.logger.Debug("Updated Samba password", "username", username)
	return nil
}
