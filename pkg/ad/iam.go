// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ad

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/errors"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Default configuration constants.
// These may be overridden by values loaded from config.
const (
	defaultLDAPPort     = "636"
	defaultBaseDNString = "ad.strata.internal"
	defaultBaseDN       = "DC=ad,DC=strata,DC=internal"
	defaultUserOU       = "OU=StrataUsers," + defaultBaseDN
	defaultGroupOU      = "OU=StrataGroups," + defaultBaseDN
	defaultComputerOU   = "OU=StrataComputers," + defaultBaseDN
	defaultAdminDN      = "CN=Administrator,CN=Users," + defaultBaseDN
)

var (
	getUserAttrsString = []string{
		"dn",
		"cn",
		"sAMAccountName",
		"userPrincipalName",
		"givenName",
		"sn",
		"description",
		"mail",
		"displayName",
		"title",
		"department",
		"company",
		"telephoneNumber",
		"mobile",
		"manager",
		"employeeID",
		"whenCreated",
		"whenChanged",
		"pwdLastSet",
		"lastLogon",
		"lastLogoff",
		"badPwdCount",
		"badPasswordTime",
		"accountExpires",
		"memberOf",
		"userAccountControl",
	}

	getGroupAttrsString = []string{
		"dn", "cn", "sAMAccountName", "description",
		"displayName", "mail", "groupType",
		"managedBy", "member", "userAccountControl",
	}

	getComputerAttrsString = []string{
		"dn", "cn", "sAMAccountName", "description",
		"dNSHostName", "operatingSystem", "operatingSystemVersion",
		"operatingSystemServicePack", "lastLogon", "userAccountControl",
		"memberOf", "location", "managedBy",
	}
)

// ADClient is a client for performing LDAP operations against Samba AD DC.
type ADClient struct {
	ldapURL    string
	realm      string
	baseDN     string
	userOU     string
	groupOU    string
	computerOU string
	adminDN    string
	adminPwd   string
	conn       *ldap.Conn
	tlsConfig  *tls.Config
	mu         sync.RWMutex
}

// User represents a Samba AD user with common attributes.
type User struct {
	CN                string // Common Name; used to form the DN.
	SAMAccountName    string
	UserPrincipalName string
	GivenName         string
	Surname           string
	Description       string
	Password          string // Plaintext password (only used when creating/updating)
	Mail              string // email address
	DisplayName       string // full name for display
	Title             string // job title
	Department        string // organizational department
	Company           string // company name
	PhoneNumber       string // telephoneNumber attribute
	Mobile            string // mobile number
	Manager           string // DN of manager
	EmployeeID        string // employee identifier
	Enabled           bool   // account status
}

// Group represents an AD group. Extend with additional fields as needed.
type Group struct {
	CN             string
	SAMAccountName string
	Description    string
	DisplayName    string   // friendly name
	Mail           string   // group email address
	GroupType      int      // security vs distribution group
	Members        []string // DNs of group members
	Scope          string   // DomainLocal, Global, Universal
	Managed        bool     // if group is managed
}

// Computer represents an AD computer object.
type Computer struct {
	CN             string
	SAMAccountName string
	Description    string
	DNSHostName    string    // FQDN
	Password       string    // Plaintext password (only used when creating/updating)
	OSName         string    // operating system name
	OSVersion      string    // OS version
	ServicePack    string    // installed service pack
	LastLogon      time.Time // last logon timestamp
	Enabled        bool      // account status
	Location       string    // physical location
	ManagedBy      string    // DN of responsible admin
}

// New initializes a new ADClient. It reads admin credentials from the app
// configuration and establishes a secure (LDAPS) connection.
func New() (*ADClient, error) {
	cfg := config.GetConfig() // Expected to have cfg.AD.AdminPassword

	// Determine LDAP connection parameters
	var ldapURL string

	// If LDAPURL is explicitly set in config, use that
	if cfg.AD.LDAPURL != "" {
		ldapURL = cfg.AD.LDAPURL
		// Ensure we have a protocol prefix
		if !strings.HasPrefix(ldapURL, "ldaps://") && !strings.HasPrefix(ldapURL, "ldap://") {
			ldapURL = fmt.Sprintf("ldaps://%s:%s", ldapURL, defaultLDAPPort)
		}
	} else if cfg.AD.DC.Enabled {
		// When DC service is enabled, use the configured hostname and realm
		ldapURL = fmt.Sprintf("ldaps://%s.%s:%s",
			strings.ToUpper(cfg.AD.DC.Hostname),
			strings.ToLower(cfg.AD.DC.Realm),
			defaultLDAPPort)
	} else {
		// Fallback to a default connection to localhost
		ldapURL = fmt.Sprintf("ldaps://localhost:%s", defaultLDAPPort)
	}

	tlsConfig := &tls.Config{
		// TODO: For testing only. In production, properly verify the server certificate.
		InsecureSkipVerify: true,
	}

	conn, err := ldap.DialURL(ldapURL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.Wrap(err, errors.ADConnectFailed)
	}

	// Use the configured Realm to build the baseDN if not explicitly set
	// TODO: validate baseDN format
	baseDN := cfg.AD.BaseDN
	realm := cfg.AD.Realm
	if cfg.AD.DC.Realm != "" {
		realm = cfg.AD.DC.Realm
	}
	if baseDN == "" && realm != "" {
		// Build baseDN from realm (e.g., "ad.strata.internal" -> "DC=ad,DC=strata,DC=internal")
		parts := strings.Split(strings.ToLower(realm), ".")
		if len(parts) > 0 {
			baseDNParts := make([]string, 0, len(parts))
			for _, part := range parts {
				baseDNParts = append(baseDNParts, fmt.Sprintf("DC=%s", part))
			}
			baseDN = strings.Join(baseDNParts, ",")
		}
	}

	// Use configured values or retrieve from server
	if baseDN == "" {
		var err error
		baseDN, err = GetDefaultNamingContext(conn)
		if err != nil {
			return nil, err
		}
	}

	// Determine admin DN
	adminDN := cfg.AD.AdminDN
	if adminDN == "" {
		adminDN = fmt.Sprintf("CN=Administrator,CN=Users,%s", baseDN)
	}

	// Bind as administrator
	if err := conn.Bind(adminDN, cfg.AD.AdminPassword); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, errors.ADInvalidCredentials)
	}

	// Set up organizational units using config or defaults
	userOU := baseDN
	if cfg.AD.UserOU != "" {
		userOU = fmt.Sprintf("%s,%s", cfg.AD.UserOU, baseDN)
	} else {
		userOU = fmt.Sprintf("OU=StrataUsers,%s", baseDN)
	}

	groupOU := baseDN
	if cfg.AD.GroupOU != "" {
		groupOU = fmt.Sprintf("%s,%s", cfg.AD.GroupOU, baseDN)
	} else {
		groupOU = fmt.Sprintf("OU=StrataGroups,%s", baseDN)
	}

	computerOU := baseDN
	if cfg.AD.ComputerOU != "" {
		computerOU = fmt.Sprintf("%s,%s", cfg.AD.ComputerOU, baseDN)
	} else {
		computerOU = fmt.Sprintf("OU=StrataComputers,%s", baseDN)
	}

	client := &ADClient{
		ldapURL:    ldapURL,
		realm:      realm,
		baseDN:     baseDN,
		userOU:     userOU,
		groupOU:    groupOU,
		computerOU: computerOU,
		adminDN:    adminDN,
		adminPwd:   cfg.AD.AdminPassword,
		conn:       conn,
		tlsConfig:  tlsConfig,
	}
	// Ensure required OUs exist
	if err := client.EnsureOUExists(client.userOU); err != nil {
		return nil, errors.Wrap(err, errors.ADCreateOUFailed).
			WithMetadata("ou_dn", client.userOU)
	}
	if err := client.EnsureOUExists(client.groupOU); err != nil {
		return nil, errors.Wrap(err, errors.ADCreateOUFailed).
			WithMetadata("ou_dn", client.groupOU)
	}
	if err := client.EnsureOUExists(client.computerOU); err != nil {
		return nil, errors.Wrap(err, errors.ADCreateOUFailed).
			WithMetadata("ou_dn", client.computerOU)
	}

	return client, nil
}

// Close terminates the LDAP connection.
func (c *ADClient) Close() {
	c.conn.Close()
}

// Reconnect re-establishes the LDAP connection
func (c *ADClient) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection if present
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	// Re-establish connection
	conn, err := ldap.DialURL(c.ldapURL, ldap.DialWithTLSConfig(c.tlsConfig))
	if err != nil {
		return errors.Wrap(err, errors.ADConnectFailed)
	}

	// Re-bind as administrator
	if err := conn.Bind(c.adminDN, c.adminPwd); err != nil {
		conn.Close()
		return errors.Wrap(err, errors.ADInvalidCredentials)
	}

	c.conn = conn
	return nil
}

// isConnectionError detects if the error is related to connection issues
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "Network Error") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "connection reset")
}

// withLDAPRetry executes LDAP operations with automatic reconnection
func (c *ADClient) withLDAPRetry(op func() error) error {
	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Add exponential backoff for retries
			delay := time.Duration(100*(1<<attempt)) * time.Millisecond // 100ms, 200ms, 400ms
			time.Sleep(delay)
		}

		// Check connection status
		c.mu.RLock()
		connOK := c.conn != nil
		c.mu.RUnlock()

		if !connOK {
			if err := c.Reconnect(); err != nil {
				return err
			}
		}

		// Try the operation
		err := op()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// If it's a connection error, reconnect and retry
		if isConnectionError(err) {
			if err := c.Reconnect(); err != nil {
				return err // Failed to reconnect
			}
			continue
		}

		// Not a connection error, return immediately
		return err
	}

	return lastErr
}

func GetDefaultNamingContext(conn *ldap.Conn) (string, error) {
	searchReq := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)
	sr, err := conn.Search(searchReq)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve RootDSE: %v", err)
	}
	if len(sr.Entries) == 0 {
		return "", fmt.Errorf("no entries returned for RootDSE")
	}
	return sr.Entries[0].GetAttributeValue("defaultNamingContext"), nil
}

// EnsureOUExists checks if an OU exists in its parent container and creates it if missing.
func (c *ADClient) EnsureOUExists(ouDN string) error {
	// Split ouDN into RDN and parent DN.
	rdnParts := strings.SplitN(ouDN, ",", 2)
	if len(rdnParts) < 2 {
		return fmt.Errorf("invalid OU DN: %s", ouDN)
	}
	rdn := rdnParts[0]      // e.g. "OU=StrataUsers"
	parentDN := rdnParts[1] // e.g. "DC=ad,DC=strata,DC=internal"

	// Build a filter to search for the OU in its parent.
	filter := fmt.Sprintf("(%s)", rdn)
	searchReq := ldap.NewSearchRequest(
		parentDN,
		ldap.ScopeSingleLevel, // only immediate children
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"dn", "ou"},
		nil,
	)

	return c.withLDAPRetry(func() error {
		c.mu.RLock()
		sr, err := c.conn.Search(searchReq)
		c.mu.RUnlock()

		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("ou_dn", ouDN).
				WithMetadata("action", "check_exists")
		}

		// If OU doesn't exist, create it.
		if len(sr.Entries) == 0 {
			// Extract the OU name from the RDN.
			rdnKV := strings.SplitN(rdn, "=", 2)
			if len(rdnKV) != 2 {
				return fmt.Errorf("invalid RDN for OU: %s", rdn)
			}
			ouName := rdnKV[1]
			addReq := ldap.NewAddRequest(ouDN, nil)
			addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})
			addReq.Attribute("ou", []string{ouName})

			c.mu.Lock()
			err := c.conn.Add(addReq)
			c.mu.Unlock()

			if err != nil {
				return errors.Wrap(err, errors.ADCreateOUFailed).
					WithMetadata("ou_dn", ouDN).
					WithMetadata("ou_name", ouName)
			}
		}
		return nil
	})
}

// CreateUser adds a new user and sets the password.
// Password modifications require an encrypted connection.
func (c *ADClient) CreateUser(user *User) error {
	if err := c.EnsureOUExists(c.userOU); err != nil {
		return errors.Wrap(err, errors.ADCreateUserFailed).
			WithMetadata("user_cn", user.CN).
			WithMetadata("ou_dn", c.userOU)
	}
	userDN := fmt.Sprintf("CN=%s,%s", user.CN, c.userOU)
	addReq := ldap.NewAddRequest(userDN, nil)
	// Set required object classes.
	addReq.Attribute(
		"objectClass",
		[]string{"top", "person", "organizationalPerson", "user"},
	)
	addReq.Attribute("cn", []string{user.CN})
	addReq.Attribute("sAMAccountName", []string{user.SAMAccountName})
	// Use provided UPN or default to SAMAccountName@domain.
	if user.UserPrincipalName != "" {
		addReq.Attribute("userPrincipalName", []string{user.UserPrincipalName})
	} else {
		addReq.Attribute("userPrincipalName", []string{fmt.Sprintf("%s@%s", user.SAMAccountName, c.realm)})
	}
	if user.GivenName != "" {
		addReq.Attribute("givenName", []string{user.GivenName})
	}
	if user.Surname != "" {
		addReq.Attribute("sn", []string{user.Surname})
	}
	if user.Description != "" {
		addReq.Attribute("description", []string{user.Description})
	}
	if user.Mail != "" {
		addReq.Attribute("mail", []string{user.Mail})
	}
	if user.DisplayName != "" {
		addReq.Attribute("displayName", []string{user.DisplayName})
	}
	if user.Title != "" {
		addReq.Attribute("title", []string{user.Title})
	}
	if user.Department != "" {
		addReq.Attribute("department", []string{user.Department})
	}
	if user.Company != "" {
		addReq.Attribute("company", []string{user.Company})
	}
	if user.PhoneNumber != "" {
		addReq.Attribute("telephoneNumber", []string{user.PhoneNumber})
	}
	if user.Mobile != "" {
		addReq.Attribute("mobile", []string{user.Mobile})
	}
	if user.Manager != "" {
		addReq.Attribute("manager", []string{user.Manager})
	}
	if user.EmployeeID != "" {
		addReq.Attribute("employeeID", []string{user.EmployeeID})
	}

	// Create the user
	err := c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Add(addReq); err != nil {
			return errors.Wrap(err, errors.ADCreateUserFailed).
				WithMetadata("user_cn", user.CN)
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Set password if provided.
	if user.Password != "" {
		quotedPwd := fmt.Sprintf("\"%s\"", user.Password)
		utf16Pwd, err := encodePassword(quotedPwd)
		if err != nil {
			return errors.Wrap(err, errors.ADEncodePasswordFailed)
		}

		modPwdReq := ldap.NewModifyRequest(userDN, nil)
		modPwdReq.Replace("unicodePwd", []string{utf16Pwd})

		err = c.withLDAPRetry(func() error {
			c.mu.Lock()
			defer c.mu.Unlock()

			if err := c.conn.Modify(modPwdReq); err != nil {
				return errors.Wrap(err, errors.ADSetPasswordFailed).
					WithMetadata("user_cn", user.CN)
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	// Enable account if requested
	if user.Enabled {
		modEnableReq := ldap.NewModifyRequest(userDN, nil)
		modEnableReq.Replace("userAccountControl", []string{"512"})

		return c.withLDAPRetry(func() error {
			c.mu.Lock()
			defer c.mu.Unlock()

			if err := c.conn.Modify(modEnableReq); err != nil {
				return errors.Wrap(err, errors.ADEnableAccountFailed).
					WithMetadata("user_cn", user.CN)
			}
			return nil
		})
	}

	return nil
}

// SearchUser returns LDAP entries for users matching the provided sAMAccountName.
func (c *ADClient) SearchUser(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf(
		"(&(objectClass=user)(sAMAccountName=%s))",
		sAMAccountName,
	)
	searchReq := ldap.NewSearchRequest(
		c.userOU,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		getUserAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("sam_account_name", sAMAccountName)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (c *ADClient) GetUserGroups(sAMAccountName string) ([]string, error) {
	filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.userOU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"memberOf"},
		nil,
	)

	var groups []string
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("sam_account_name", sAMAccountName)
		}

		if len(sr.Entries) == 0 {
			return errors.New(errors.ADUserNotFound, "Invalid user").
				WithMetadata("sam_account_name", sAMAccountName)
		}

		groups = sr.Entries[0].GetAttributeValues("memberOf")
		return nil
	})

	if err != nil {
		return nil, err
	}

	return groups, nil
}

// ListUsers retrieves all user entries in the user OU.
func (c *ADClient) ListUsers() ([]*ldap.Entry, error) {
	filter := "(objectClass=user)"
	searchReq := ldap.NewSearchRequest(
		c.userOU,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		getUserAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("scope", "all_users").
				WithMetadata("base_dn", c.userOU)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ListGroups retrieves all group entries in the group OU.
func (c *ADClient) ListGroups() ([]*ldap.Entry, error) {
	filter := "(objectClass=group)"
	searchReq := ldap.NewSearchRequest(
		c.groupOU,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		getGroupAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("scope", "all_groups").
				WithMetadata("base_dn", c.groupOU)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ListComputers retrieves all computer entries in the computer OU.
func (c *ADClient) ListComputers() ([]*ldap.Entry, error) {
	filter := "(objectClass=computer)"
	searchReq := ldap.NewSearchRequest(
		c.computerOU,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		getComputerAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("scope", "all_computers").
				WithMetadata("base_dn", c.baseDN)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// UpdateUser updates the attributes of an existing user.
// Only non-empty fields in the provided User struct will be updated.
func (c *ADClient) UpdateUser(user *User) error {
	userDN := fmt.Sprintf("CN=%s,%s", user.CN, c.userOU)
	modReq := ldap.NewModifyRequest(userDN, nil)
	// Update each attribute only if a new value is provided.
	if user.GivenName != "" {
		modReq.Replace("givenName", []string{user.GivenName})
	}
	if user.Surname != "" {
		modReq.Replace("sn", []string{user.Surname})
	}
	if user.Description != "" {
		modReq.Replace("description", []string{user.Description})
	}
	if user.Mail != "" {
		modReq.Replace("mail", []string{user.Mail})
	}
	if user.DisplayName != "" {
		modReq.Replace("displayName", []string{user.DisplayName})
	}
	if user.Title != "" {
		modReq.Replace("title", []string{user.Title})
	}
	if user.Department != "" {
		modReq.Replace("department", []string{user.Department})
	}
	if user.Company != "" {
		modReq.Replace("company", []string{user.Company})
	}
	if user.PhoneNumber != "" {
		modReq.Replace("telephoneNumber", []string{user.PhoneNumber})
	}
	if user.Mobile != "" {
		modReq.Replace("mobile", []string{user.Mobile})
	}
	if user.Manager != "" {
		modReq.Replace("manager", []string{user.Manager})
	}
	if user.EmployeeID != "" {
		modReq.Replace("employeeID", []string{user.EmployeeID})
	}
	if user.Enabled {
		modReq.Replace("userAccountControl", []string{"512"})
	} else if !user.Enabled {
		modReq.Replace("userAccountControl", []string{"514"})
	}

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modReq); err != nil {
			return errors.Wrap(err, errors.ADUpdateUserFailed).
				WithMetadata("user_cn", user.CN)
		}
		return nil
	})
}

// DeleteUser removes a user identified by their Common Name.
func (c *ADClient) DeleteUser(cn string) error {
	userDN := fmt.Sprintf("CN=%s,%s", cn, c.userOU)
	delReq := ldap.NewDelRequest(userDN, nil)

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Del(delReq); err != nil {
			return errors.Wrap(err, errors.ADDeleteUserFailed).
				WithMetadata("user_cn", cn)
		}
		return nil
	})
}

// CreateGroup creates a new group in AD.
func (c *ADClient) CreateGroup(group *Group) error {
	// EnsureOUExists already uses withLDAPRetry
	if err := c.EnsureOUExists(c.groupOU); err != nil {
		return errors.Wrap(err, errors.ADCreateUserFailed).
			WithMetadata("group_cn", group.CN).
			WithMetadata("ou_dn", c.groupOU)
	}

	groupDN := fmt.Sprintf("CN=%s,%s", group.CN, c.groupOU)
	addReq := ldap.NewAddRequest(groupDN, nil)
	addReq.Attribute("objectClass", []string{"top", "group"})
	addReq.Attribute("cn", []string{group.CN})
	addReq.Attribute("sAMAccountName", []string{group.SAMAccountName})

	if group.Description != "" {
		addReq.Attribute("description", []string{group.Description})
	}
	if group.DisplayName != "" {
		addReq.Attribute("displayName", []string{group.DisplayName})
	}
	if group.Mail != "" {
		addReq.Attribute("mail", []string{group.Mail})
	}
	if group.GroupType != 0 {
		addReq.Attribute("groupType", []string{fmt.Sprintf("%d", group.GroupType)})
	}
	if len(group.Members) > 0 {
		addReq.Attribute("member", group.Members)
	}

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Add(addReq); err != nil {
			return errors.Wrap(err, errors.ADCreateGroupFailed).
				WithMetadata("group_cn", group.CN)
		}
		return nil
	})
}

// SearchGroup returns LDAP entries for groups matching the provided sAMAccountName.
func (c *ADClient) SearchGroup(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.groupOU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		getGroupAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("sam_account_name", sAMAccountName)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// GetGroupMembers returns the members of a group
func (c *ADClient) GetGroupMembers(sAMAccountName string) ([]string, error) {
	filter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.groupOU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"member"},
		nil,
	)

	var members []string
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("sam_account_name", sAMAccountName)
		}

		if len(sr.Entries) == 0 {
			return errors.New(errors.ADGroupNotFound, "Invalid group").
				WithMetadata("sam_account_name", sAMAccountName)
		}

		members = sr.Entries[0].GetAttributeValues("member")
		return nil
	})

	if err != nil {
		return nil, err
	}

	return members, nil
}

// UpdateGroup updates the attributes of an existing group.
func (c *ADClient) UpdateGroup(group *Group) error {
	groupDN := fmt.Sprintf("CN=%s,%s", group.CN, c.groupOU)
	modReq := ldap.NewModifyRequest(groupDN, nil)

	if group.Description != "" {
		modReq.Replace("description", []string{group.Description})
	}
	if group.DisplayName != "" {
		modReq.Replace("displayName", []string{group.DisplayName})
	}
	if group.Mail != "" {
		modReq.Replace("mail", []string{group.Mail})
	}
	if group.GroupType != 0 {
		modReq.Replace("groupType", []string{fmt.Sprintf("%d", group.GroupType)})
	}

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modReq); err != nil {
			return errors.Wrap(err, errors.ADUpdateGroupFailed).
				WithMetadata("group_cn", group.CN)
		}
		return nil
	})
}

// AddMembersToGroup adds members to a group.
func (c *ADClient) AddMembersToGroup(membersDN []string, groupCN string) error {
	groupDN := fmt.Sprintf("CN=%s,%s", groupCN, c.groupOU)
	modReq := ldap.NewModifyRequest(groupDN, nil)
	modReq.Add("member", membersDN)

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modReq); err != nil {
			return errors.Wrap(err, errors.ADUpdateGroupFailed).
				WithMetadata("group_cn", groupCN).
				WithMetadata("action", "add_members").
				WithMetadata("members_count", fmt.Sprintf("%d", len(membersDN)))
		}
		return nil
	})
}

// RemoveMembersFromGroup removes members from a group.
func (c *ADClient) RemoveMembersFromGroup(membersDN []string, groupCN string) error {
	groupDN := fmt.Sprintf("CN=%s,%s", groupCN, c.groupOU)
	modReq := ldap.NewModifyRequest(groupDN, nil)
	modReq.Delete("member", membersDN)

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modReq); err != nil {
			return errors.Wrap(err, errors.ADUpdateGroupFailed).
				WithMetadata("group_cn", groupCN).
				WithMetadata("action", "remove_members").
				WithMetadata("members_count", fmt.Sprintf("%d", len(membersDN)))
		}
		return nil
	})
}

// DeleteGroup removes a group.
func (c *ADClient) DeleteGroup(cn string) error {
	groupDN := fmt.Sprintf("CN=%s,%s", cn, c.groupOU)
	delReq := ldap.NewDelRequest(groupDN, nil)

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Del(delReq); err != nil {
			return errors.Wrap(err, errors.ADDeleteGroupFailed).
				WithMetadata("group_cn", cn)
		}
		return nil
	})
}

// GetUserOU returns the configured User OU for testing purposes
func (c *ADClient) GetUserOU() string {
	return c.userOU
}

// GetGroupOU returns the configured Group OU for testing purposes
func (c *ADClient) GetGroupOU() string {
	return c.groupOU
}

// SearchComputer returns LDAP entries for computers matching the sAMAccountName.
func (c *ADClient) SearchComputer(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=computer)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.computerOU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		getComputerAttrsString,
		nil,
	)

	var entries []*ldap.Entry
	err := c.withLDAPRetry(func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()

		sr, err := c.conn.Search(searchReq)
		if err != nil {
			return errors.Wrap(err, errors.ADSearchFailed).
				WithMetadata("sam_account_name", sAMAccountName)
		}
		entries = sr.Entries
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// CreateComputer creates a new computer object in AD.
func (c *ADClient) CreateComputer(comp *Computer) error {
	if err := c.EnsureOUExists(c.computerOU); err != nil {
		return errors.Wrap(err, errors.ADCreateComputerFailed).
			WithMetadata("computer_cn", comp.CN).
			WithMetadata("ou_dn", c.computerOU)
	}

	computerDN := fmt.Sprintf("CN=%s,%s", comp.CN, c.computerOU)
	addReq := ldap.NewAddRequest(computerDN, nil)
	addReq.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "computer"})
	addReq.Attribute("cn", []string{comp.CN})
	addReq.Attribute("sAMAccountName", []string{comp.SAMAccountName})

	if comp.Description != "" {
		addReq.Attribute("description", []string{comp.Description})
	}
	if comp.DNSHostName != "" {
		addReq.Attribute("dNSHostName", []string{comp.DNSHostName})
	}
	if comp.OSName != "" {
		addReq.Attribute("operatingSystem", []string{comp.OSName})
	}
	if comp.OSVersion != "" {
		addReq.Attribute("operatingSystemVersion", []string{comp.OSVersion})
	}
	if comp.ServicePack != "" {
		addReq.Attribute("operatingSystemServicePack", []string{comp.ServicePack})
	}
	if comp.Location != "" {
		addReq.Attribute("location", []string{comp.Location})
	}
	if comp.ManagedBy != "" {
		addReq.Attribute("managedBy", []string{comp.ManagedBy})
	}

	// Create the computer object
	err := c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Add(addReq); err != nil {
			return errors.Wrap(err, errors.ADCreateComputerFailed).
				WithMetadata("computer_cn", comp.CN).
				WithMetadata("ou_dn", c.computerOU)
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Set password if provided
	if comp.Password != "" {
		quotedPwd := fmt.Sprintf("\"%s\"", comp.Password)
		utf16Pwd, err := encodePassword(quotedPwd)
		if err != nil {
			return errors.Wrap(err, errors.ADEncodePasswordFailed)
		}

		modPwdReq := ldap.NewModifyRequest(computerDN, nil)
		modPwdReq.Replace("unicodePwd", []string{utf16Pwd})

		err = c.withLDAPRetry(func() error {
			c.mu.Lock()
			defer c.mu.Unlock()

			if err := c.conn.Modify(modPwdReq); err != nil {
				return errors.Wrap(err, errors.ADSetPasswordFailed).
					WithMetadata("computer_cn", comp.CN)
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	// Enable the computer account (userAccountControl=4096 for enabled computer)
	// 4096 = WORKSTATION_TRUST_ACCOUNT
	// 4096 + 32 = WORKSTATION_TRUST_ACCOUNT and password not required
	modEnableReq := ldap.NewModifyRequest(computerDN, nil)
	modEnableReq.Replace("userAccountControl", []string{"4128"})

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modEnableReq); err != nil {
			return errors.Wrap(err, errors.ADEnableAccountFailed).
				WithMetadata("computer_cn", comp.CN)
		}
		return nil
	})
}

// UpdateComputer updates the attributes of an existing computer object.
func (c *ADClient) UpdateComputer(comp *Computer) error {
	computerDN := fmt.Sprintf("CN=%s,%s", comp.CN, c.computerOU)
	modReq := ldap.NewModifyRequest(computerDN, nil)

	if comp.Description != "" {
		modReq.Replace("description", []string{comp.Description})
	}
	if comp.DNSHostName != "" {
		modReq.Replace("dNSHostName", []string{comp.DNSHostName})
	}
	if comp.OSName != "" {
		modReq.Replace("operatingSystem", []string{comp.OSName})
	}
	if comp.OSVersion != "" {
		modReq.Replace("operatingSystemVersion", []string{comp.OSVersion})
	}
	if comp.ServicePack != "" {
		modReq.Replace("operatingSystemServicePack", []string{comp.ServicePack})
	}
	if comp.Location != "" {
		modReq.Replace("location", []string{comp.Location})
	}
	if comp.ManagedBy != "" {
		modReq.Replace("managedBy", []string{comp.ManagedBy})
	}

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Modify(modReq); err != nil {
			return errors.Wrap(err, errors.ADUpdateComputerFailed).
				WithMetadata("computer_cn", comp.CN)
		}
		return nil
	})
}

// DeleteComputer removes a computer object identified by its Common Name.
func (c *ADClient) DeleteComputer(cn string) error {
	computerDN := fmt.Sprintf("CN=%s,%s", cn, c.computerOU)
	delReq := ldap.NewDelRequest(computerDN, nil)

	return c.withLDAPRetry(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if err := c.conn.Del(delReq); err != nil {
			return errors.Wrap(err, errors.ADDeleteComputerFailed).
				WithMetadata("computer_cn", cn)
		}
		return nil
	})
}

// encodePassword converts a plaintext password (with quotes) into a UTF-16LE
// encoded string, as required by AD's unicodePwd attribute.
func encodePassword(password string) (string, error) {
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	utf16Pwd, err := transformString(encoder, password)
	if err != nil {
		return "", err
	}
	return utf16Pwd, nil
}

// transformString applies a transformer to the given string.
func transformString(t transform.Transformer, s string) (string, error) {
	result, _, err := transform.String(t, s)
	if err != nil {
		return "", err
	}
	return result, nil
}
