// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/ad"
	"github.com/stratastor/rodent/pkg/errors"
)

// ADHandler provides HTTP endpoints for AD operations
type ADHandler struct {
	client *ad.ADClient
}

// NewADHandler creates a new handler with an initialized AD client
func NewADHandler() (*ADHandler, error) {
	client, err := ad.New()
	if err != nil {
		return nil, errors.Wrap(err, errors.ADConnectFailed)
	}

	return &ADHandler{
		client: client,
	}, nil
}

// NewADHandlerWithClient creates a new handler with an existing AD client
// This is useful for gRPC handlers that share a client with the REST API
func NewADHandlerWithClient(client *ad.ADClient) *ADHandler {
	return &ADHandler{
		client: client,
	}
}

// Close closes the underlying AD client connection
func (h *ADHandler) Close() {
	if h.client != nil {
		h.client.Close()
	}
}

// User API handlers

// UserRequest is used for binding user creation/update requests
type UserRequest struct {
	CN                string `json:"cn"                  binding:"required"`
	SAMAccountName    string `json:"sam_account_name"    binding:"required"`
	UserPrincipalName string `json:"user_principal_name"`
	GivenName         string `json:"given_name"`
	Surname           string `json:"surname"`
	Description       string `json:"description"`
	Password          string `json:"password,omitempty"` // Omitted from responses
	Mail              string `json:"mail"`
	DisplayName       string `json:"display_name"`
	Title             string `json:"title"`
	Department        string `json:"department"`
	Company           string `json:"company"`
	PhoneNumber       string `json:"phone_number"`
	Mobile            string `json:"mobile"`
	Manager           string `json:"manager"`
	EmployeeID        string `json:"employee_id"`
	Enabled           bool   `json:"enabled"`
}

// toADUser converts API request to AD User model
func (r *UserRequest) toADUser() *ad.User {
	return &ad.User{
		CN:                r.CN,
		SAMAccountName:    r.SAMAccountName,
		UserPrincipalName: r.UserPrincipalName,
		GivenName:         r.GivenName,
		Surname:           r.Surname,
		Description:       r.Description,
		Password:          r.Password,
		Mail:              r.Mail,
		DisplayName:       r.DisplayName,
		Title:             r.Title,
		Department:        r.Department,
		Company:           r.Company,
		PhoneNumber:       r.PhoneNumber,
		Mobile:            r.Mobile,
		Manager:           r.Manager,
		EmployeeID:        r.EmployeeID,
		Enabled:           r.Enabled,
	}
}

// CreateUser creates a new AD user
func (h *ADHandler) CreateUser(c *gin.Context) {
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	user := req.toADUser()
	if err := h.client.CreateUser(user); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

// GetUser retrieves a user by sAMAccountName
func (h *ADHandler) GetUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "username is required"))
		return
	}

	entries, err := h.client.SearchUser(username)
	if err != nil {
		APIError(c, err)
		return
	}

	if len(entries) == 0 {
		APIError(c, errors.New(errors.ADUserNotFound, "User not found"))
		return
	}

	c.JSON(http.StatusOK, entries[0])
}

// UpdateUser updates an existing AD user
func (h *ADHandler) UpdateUser(c *gin.Context) {
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	user := req.toADUser()
	if err := h.client.UpdateUser(user); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// DeleteUser removes a user by common name
func (h *ADHandler) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "username is required"))
		return
	}

	if err := h.client.DeleteUser(username); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListUsers returns all users
func (h *ADHandler) ListUsers(c *gin.Context) {
	entries, err := h.client.ListUsers()
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, entries)
}

// GetUserGroups retrieves groups a user belongs to
func (h *ADHandler) GetUserGroups(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "username is required"))
		return
	}

	groups, err := h.client.GetUserGroups(username)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
	})
}

// Group API handlers

// GroupRequest is used for binding group creation/update requests
type GroupRequest struct {
	CN             string   `json:"cn"               binding:"required"`
	SAMAccountName string   `json:"sam_account_name" binding:"required"`
	Description    string   `json:"description"`
	DisplayName    string   `json:"display_name"`
	Mail           string   `json:"mail"`
	GroupType      int      `json:"group_type"`
	Members        []string `json:"members"`
	Scope          string   `json:"scope"` // DomainLocal, Global, Universal
	Managed        bool     `json:"managed"`
}

// toADGroup converts API request to AD Group model
func (r *GroupRequest) toADGroup() *ad.Group {
	return &ad.Group{
		CN:             r.CN,
		SAMAccountName: r.SAMAccountName,
		Description:    r.Description,
		DisplayName:    r.DisplayName,
		Mail:           r.Mail,
		GroupType:      r.GroupType,
		Members:        r.Members,
		Scope:          r.Scope,
		Managed:        r.Managed,
	}
}

// CreateGroup creates a new AD group
func (h *ADHandler) CreateGroup(c *gin.Context) {
	var req GroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	group := req.toADGroup()
	if err := h.client.CreateGroup(group); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

// GetGroup retrieves a group by sAMAccountName
func (h *ADHandler) GetGroup(c *gin.Context) {
	groupname := c.Param("groupname")
	if groupname == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "groupname is required"))
		return
	}

	entries, err := h.client.SearchGroup(groupname)
	if err != nil {
		APIError(c, err)
		return
	}

	if len(entries) == 0 {
		APIError(c, errors.New(errors.ADGroupNotFound, "Group not found"))
		return
	}

	c.JSON(http.StatusOK, entries[0])
}

// UpdateGroup updates an existing AD group
func (h *ADHandler) UpdateGroup(c *gin.Context) {
	var req GroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	group := req.toADGroup()
	if err := h.client.UpdateGroup(group); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// DeleteGroup removes a group by common name
func (h *ADHandler) DeleteGroup(c *gin.Context) {
	groupname := c.Param("groupname")
	if groupname == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "groupname is required"))
		return
	}

	if err := h.client.DeleteGroup(groupname); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListGroups returns all groups
func (h *ADHandler) ListGroups(c *gin.Context) {
	entries, err := h.client.ListGroups()
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, entries)
}

// GetGroupMembers retrieves members of a group
func (h *ADHandler) GetGroupMembers(c *gin.Context) {
	groupname := c.Param("groupname")
	if groupname == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "groupname is required"))
		return
	}

	members, err := h.client.GetGroupMembers(groupname)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"members": members,
	})
}

// GroupMembershipRequest for adding/removing group members
type GroupMembershipRequest struct {
	Members []string `json:"members" binding:"required"`
}

// AddGroupMembers adds members to a group
func (h *ADHandler) AddGroupMembers(c *gin.Context) {
	groupname := c.Param("groupname")
	if groupname == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "groupname is required"))
		return
	}

	var req GroupMembershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.client.AddMembersToGroup(req.Members, groupname); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// RemoveGroupMembers removes members from a group
func (h *ADHandler) RemoveGroupMembers(c *gin.Context) {
	groupname := c.Param("groupname")
	if groupname == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "groupname is required"))
		return
	}

	var req GroupMembershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.client.RemoveMembersFromGroup(req.Members, groupname); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// Computer API handlers

// ComputerRequest is used for binding computer creation/update requests
type ComputerRequest struct {
	CN             string `json:"cn"               binding:"required"`
	SAMAccountName string `json:"sam_account_name" binding:"required"`
	Description    string `json:"description"`
	DNSHostName    string `json:"dns_hostname"` // FQDN
	OSName         string `json:"os_name"`      // operating system name
	OSVersion      string `json:"os_version"`   // OS version
	ServicePack    string `json:"service_pack"` // installed service pack
	Location       string `json:"location"`     // physical location
	ManagedBy      string `json:"managed_by"`   // DN of responsible admin
	Enabled        bool   `json:"enabled"`
}

// toADComputer converts API request to AD Computer model
func (r *ComputerRequest) toADComputer() *ad.Computer {
	return &ad.Computer{
		CN:             r.CN,
		SAMAccountName: r.SAMAccountName,
		Description:    r.Description,
		DNSHostName:    r.DNSHostName,
		OSName:         r.OSName,
		OSVersion:      r.OSVersion,
		ServicePack:    r.ServicePack,
		Location:       r.Location,
		ManagedBy:      r.ManagedBy,
		Enabled:        r.Enabled,
	}
}

// CreateComputer creates a new AD computer
func (h *ADHandler) CreateComputer(c *gin.Context) {
	var req ComputerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	computer := req.toADComputer()
	if err := h.client.CreateComputer(computer); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

// GetComputer retrieves a computer by sAMAccountName
func (h *ADHandler) GetComputer(c *gin.Context) {
	computername := c.Param("computername")
	if computername == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "computername is required"))
		return
	}

	entries, err := h.client.SearchComputer(computername)
	if err != nil {
		APIError(c, err)
		return
	}

	if len(entries) == 0 {
		// Computer objects not found
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	// TODO: entries must be destructured; ldap.Entry to a json friendly struct
	c.JSON(http.StatusOK, entries)
}

func (h *ADHandler) ListComputers(c *gin.Context) {
	entries, err := h.client.ListComputers()
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, entries)
}

// UpdateComputer updates an existing AD computer
func (h *ADHandler) UpdateComputer(c *gin.Context) {
	var req ComputerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	computer := req.toADComputer()
	if err := h.client.UpdateComputer(computer); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// DeleteComputer removes a computer by common name
func (h *ADHandler) DeleteComputer(c *gin.Context) {
	computername := c.Param("computername")
	if computername == "" {
		APIError(c, errors.New(errors.ServerRequestValidation, "computername is required"))
		return
	}

	if err := h.client.DeleteComputer(computername); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// APIError is a helper to add structured errors to context
func APIError(c *gin.Context, err error) {
	c.Error(err)
	c.Abort()
}
