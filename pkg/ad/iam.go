// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package ad

import (
	"crypto/tls"
	"fmt"
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
	defaultLDAPURL = "ldaps://DC1.ad.strata.internal:636"
	defaultBaseDN  = "DC=ad,DC=strata,DC=internal"
	defaultUserOU  = "CN=Users," + defaultBaseDN
	defaultGroupOU = "CN=Users," + defaultBaseDN
	defaultAdminDN = "CN=Administrator,CN=Users," + defaultBaseDN
)

// ADClient is a client for performing LDAP operations against Samba AD DC.
type ADClient struct {
	ldapURL   string
	baseDN    string
	userOU    string
	groupOU   string
	adminDN   string
	conn      *ldap.Conn
	tlsConfig *tls.Config
}

// New initializes a new ADClient. It reads admin credentials from the app
// configuration and establishes a secure (LDAPS) connection.
func New() (*ADClient, error) {
	cfg := config.GetConfig() // Expected to have cfg.AD.AdminPassword
	tlsConfig := &tls.Config{
		// TODO: For testing only. In production, properly verify the server certificate.
		InsecureSkipVerify: true,
	}
	conn, err := ldap.DialURL(defaultLDAPURL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.Wrap(err, errors.ADConnectFailed)
	}
	// Bind as administrator.
	if err := conn.Bind(defaultAdminDN, cfg.AD.AdminPassword); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, errors.ADInvalidCredentials)
	}
	client := &ADClient{
		ldapURL:   defaultLDAPURL,
		baseDN:    defaultBaseDN,
		userOU:    defaultUserOU,
		groupOU:   defaultGroupOU,
		adminDN:   defaultAdminDN,
		conn:      conn,
		tlsConfig: tlsConfig,
	}
	return client, nil
}

// Close terminates the LDAP connection.
func (c *ADClient) Close() {
	c.conn.Close()
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

// CreateUser adds a new user and sets the password.
// Password modifications require an encrypted connection.
func (c *ADClient) CreateUser(user *User) error {
	userDN := fmt.Sprintf("CN=%s,%s", user.CN, c.userOU)
	addReq := ldap.NewAddRequest(userDN, nil)
	// Set required object classes.
	addReq.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "user"})
	addReq.Attribute("cn", []string{user.CN})
	addReq.Attribute("sAMAccountName", []string{user.SAMAccountName})
	// Use provided UPN or default to SAMAccountName@domain.
	if user.UserPrincipalName != "" {
		addReq.Attribute("userPrincipalName", []string{user.UserPrincipalName})
	} else {
		addReq.Attribute("userPrincipalName", []string{fmt.Sprintf("%s@ad.strata.internal", user.SAMAccountName)})
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

	if err := c.conn.Add(addReq); err != nil {
		return errors.Wrap(err, errors.ADCreateUserFailed).
			WithMetadata("user_cn", user.CN)
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
		if err := c.conn.Modify(modPwdReq); err != nil {
			return errors.Wrap(err, errors.ADSetPasswordFailed).
				WithMetadata("user_cn", user.CN)
		}
	}

	// Enable account (userAccountControl=512).
	modEnableReq := ldap.NewModifyRequest(userDN, nil)
	modEnableReq.Replace("userAccountControl", []string{"512"})
	if err := c.conn.Modify(modEnableReq); err != nil {
		return errors.Wrap(err, errors.ADEnableAccountFailed).
			WithMetadata("user_cn", user.CN)
	}
	return nil
}

// SearchUser returns LDAP entries for users matching the provided sAMAccountName.
func (c *ADClient) SearchUser(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.userOU,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{
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
		},
		nil,
	)
	sr, err := c.conn.Search(searchReq)
	if err != nil {
		return nil, errors.Wrap(err, errors.ADSearchFailed).
			WithMetadata("sam_account_name", sAMAccountName)
	}
	return sr.Entries, nil
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
	if err := c.conn.Modify(modReq); err != nil {
		return errors.Wrap(err, errors.ADUpdateUserFailed).
			WithMetadata("user_cn", user.CN)
	}
	return nil
}

// DeleteUser removes a user identified by their Common Name.
func (c *ADClient) DeleteUser(cn string) error {
	userDN := fmt.Sprintf("CN=%s,%s", cn, c.userOU)
	delReq := ldap.NewDelRequest(userDN, nil)
	if err := c.conn.Del(delReq); err != nil {
		return errors.Wrap(err, errors.ADDeleteUserFailed).
			WithMetadata("user_cn", cn)
	}
	return nil
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
	MemberOf       []string // DNs of parent groups
	Scope          string   // DomainLocal, Global, Universal
	Managed        bool     // if group is managed
}

// CreateGroup creates a new group in AD.
func (c *ADClient) CreateGroup(group *Group) error {
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
	if len(group.MemberOf) > 0 {
		addReq.Attribute("memberOf", group.MemberOf)
	}

	if err := c.conn.Add(addReq); err != nil {
		return errors.Wrap(err, errors.ADCreateGroupFailed).
			WithMetadata("group_cn", group.CN)
	}
	return nil
}

// SearchGroup returns LDAP entries for groups matching the provided sAMAccountName.
func (c *ADClient) SearchGroup(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=group)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.groupOU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{
			"dn", "cn", "sAMAccountName", "description",
			"displayName", "mail", "groupType", "member",
			"memberOf", "managedBy",
		},
		nil,
	)
	sr, err := c.conn.Search(searchReq)
	if err != nil {
		return nil, errors.Wrap(err, errors.ADSearchFailed).
			WithMetadata("sam_account_name", sAMAccountName)
	}
	return sr.Entries, nil
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
	if len(group.Members) > 0 {
		modReq.Replace("member", group.Members)
	}
	if len(group.MemberOf) > 0 {
		modReq.Replace("memberOf", group.MemberOf)
	}

	if err := c.conn.Modify(modReq); err != nil {
		return errors.Wrap(err, errors.ADUpdateGroupFailed).
			WithMetadata("group_cn", group.CN)
	}
	return nil
}

// DeleteGroup removes a group identified by its Common Name.
func (c *ADClient) DeleteGroup(cn string) error {
	groupDN := fmt.Sprintf("CN=%s,%s", cn, c.groupOU)
	delReq := ldap.NewDelRequest(groupDN, nil)
	if err := c.conn.Del(delReq); err != nil {
		return errors.Wrap(err, errors.ADDeleteGroupFailed).
			WithMetadata("group_cn", cn)
	}
	return nil
}

// Computer represents an AD computer object.
type Computer struct {
	CN             string
	SAMAccountName string
	Description    string
	DNSHostName    string    // FQDN
	OSName         string    // operating system name
	OSVersion      string    // OS version
	ServicePack    string    // installed service pack
	LastLogon      time.Time // last logon timestamp
	Enabled        bool      // account status
	MemberOf       []string  // DNs of groups
	Location       string    // physical location
	ManagedBy      string    // DN of responsible admin
}

// CreateComputer creates a new computer object in AD.
func (c *ADClient) CreateComputer(comp *Computer) error {
	computerDN := fmt.Sprintf("CN=%s,%s", comp.CN, c.baseDN)
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
	if len(comp.MemberOf) > 0 {
		addReq.Attribute("memberOf", comp.MemberOf)
	}
	if comp.Location != "" {
		addReq.Attribute("location", []string{comp.Location})
	}
	if comp.ManagedBy != "" {
		addReq.Attribute("managedBy", []string{comp.ManagedBy})
	}

	if err := c.conn.Add(addReq); err != nil {
		return errors.Wrap(err, errors.ADCreateComputerFailed).
			WithMetadata("computer_cn", comp.CN)
	}
	return nil
}

// SearchComputer returns LDAP entries for computers matching the provided sAMAccountName.
func (c *ADClient) SearchComputer(sAMAccountName string) ([]*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=computer)(sAMAccountName=%s))", sAMAccountName)
	searchReq := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{
			"dn", "cn", "sAMAccountName", "description",
			"dNSHostName", "operatingSystem", "operatingSystemVersion",
			"operatingSystemServicePack", "lastLogon", "userAccountControl",
			"memberOf", "location", "managedBy",
		},
		nil,
	)
	sr, err := c.conn.Search(searchReq)
	if err != nil {
		return nil, errors.Wrap(err, errors.ADSearchFailed).
			WithMetadata("sam_account_name", sAMAccountName)
	}
	return sr.Entries, nil
}

// UpdateComputer updates the attributes of an existing computer object.
func (c *ADClient) UpdateComputer(comp *Computer) error {
	computerDN := fmt.Sprintf("CN=%s,%s", comp.CN, c.baseDN)
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
	if len(comp.MemberOf) > 0 {
		modReq.Replace("memberOf", comp.MemberOf)
	}
	if comp.Location != "" {
		modReq.Replace("location", []string{comp.Location})
	}
	if comp.ManagedBy != "" {
		modReq.Replace("managedBy", []string{comp.ManagedBy})
	}

	if err := c.conn.Modify(modReq); err != nil {
		return errors.Wrap(err, errors.ADUpdateComputerFailed).
			WithMetadata("computer_cn", comp.CN)
	}
	return nil
}

// DeleteComputer removes a computer object identified by its Common Name.
func (c *ADClient) DeleteComputer(cn string) error {
	computerDN := fmt.Sprintf("CN=%s,%s", cn, c.baseDN)
	delReq := ldap.NewDelRequest(computerDN, nil)
	if err := c.conn.Del(delReq); err != nil {
		return errors.Wrap(err, errors.ADDeleteComputerFailed).
			WithMetadata("computer_cn", cn)
	}
	return nil
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
