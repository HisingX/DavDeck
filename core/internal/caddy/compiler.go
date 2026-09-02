// Package caddy owns deterministic managed Caddy configuration generation.
package caddy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"davdeck.dev/davdeck/core/internal/dnsprovider"
	"davdeck.dev/davdeck/core/internal/domain"
)

var readMethods = []string{"GET", "HEAD", "OPTIONS", "PROPFIND"}

type ShareWithPermissions = domain.ShareWithPermissions
type RuntimeConfigInput = domain.RuntimeConfigInput

type CompiledConfig struct {
	JSON     []byte
	SHA256   string
	Warnings []string
}

type Compiler struct{}

func (Compiler) Compile(input RuntimeConfigInput) (CompiledConfig, error) {
	if err := input.ServerSettings.Validate(); err != nil {
		return CompiledConfig{}, fmt.Errorf("validate server settings: %w", err)
	}
	hostname := ""
	providers := make(map[domain.ID]domain.DNSProviderCredential, len(input.DNSProviderCredentials))
	for _, provider := range input.DNSProviderCredentials {
		if err := provider.Validate(); err != nil {
			return CompiledConfig{}, fmt.Errorf("validate DNS provider %s: %w", provider.ID, err)
		}
		if _, exists := providers[provider.ID]; exists {
			return CompiledConfig{}, fmt.Errorf("duplicate DNS provider id %s", provider.ID)
		}
		providers[provider.ID] = provider
	}
	if input.TLSProfile != nil {
		if err := input.TLSProfile.Validate(); err != nil {
			return CompiledConfig{}, fmt.Errorf("validate TLS profile: %w", err)
		}
		hostname = input.TLSProfile.Hostname
	}
	users := append([]domain.User(nil), input.Users...)
	sort.Slice(users, func(i, j int) bool {
		if users[i].UsernameNormalized == users[j].UsernameNormalized {
			return users[i].ID < users[j].ID
		}
		return users[i].UsernameNormalized < users[j].UsernameNormalized
	})
	userByID := make(map[domain.ID]domain.User, len(users))
	usernames := make(map[string]struct{}, len(users))
	for _, user := range users {
		if err := user.Validate(); err != nil {
			return CompiledConfig{}, fmt.Errorf("validate user %s: %w", user.ID, err)
		}
		if _, exists := userByID[user.ID]; exists {
			return CompiledConfig{}, fmt.Errorf("duplicate user id %s", user.ID)
		}
		if _, exists := usernames[user.UsernameNormalized]; exists {
			return CompiledConfig{}, fmt.Errorf("duplicate normalized username %s", user.UsernameNormalized)
		}
		userByID[user.ID] = user
		usernames[user.UsernameNormalized] = struct{}{}
	}
	shares := append([]ShareWithPermissions(nil), input.Shares...)
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Share.Slug == shares[j].Share.Slug {
			return shares[i].Share.ID < shares[j].Share.ID
		}
		return shares[i].Share.Slug < shares[j].Share.Slug
	})
	routes := make([]route, 0, len(shares)*2)
	seenShares := make(map[domain.ID]struct{}, len(shares))
	seenSlugs := make(map[string]struct{}, len(shares))
	discoveryAccounts := make(map[string]account)
	discoveryEntries := make(map[string][]discoveryEntry)
	warnings := make([]string, 0)
	for _, item := range shares {
		if err := item.Share.Validate(); err != nil {
			return CompiledConfig{}, fmt.Errorf("validate share %s: %w", item.Share.ID, err)
		}
		if _, exists := seenShares[item.Share.ID]; exists {
			return CompiledConfig{}, fmt.Errorf("duplicate share id %s", item.Share.ID)
		}
		seenShares[item.Share.ID] = struct{}{}
		if _, exists := seenSlugs[item.Share.Slug]; exists {
			return CompiledConfig{}, fmt.Errorf("duplicate share slug %s", item.Share.Slug)
		}
		seenSlugs[item.Share.Slug] = struct{}{}
		if !item.Share.Enabled {
			continue
		}
		readAccounts, writeAccounts, err := compileAccounts(item, userByID)
		if err != nil {
			return CompiledConfig{}, err
		}
		pathPrefix := joinPublicPath(input.ServerSettings.PublicBasePath, item.Share.Slug)
		paths := []string{pathPrefix, pathPrefix + "/*"}
		if len(readAccounts) > 0 {
			routes = append(routes, permissionRoute(paths, readMethods, readAccounts, item.Share.Path, pathPrefix, hostname))
			for _, readAccount := range readAccounts {
				discoveryAccounts[readAccount.Username] = readAccount
				discoveryEntries[readAccount.Username] = append(discoveryEntries[readAccount.Username], discoveryEntry{
					Slug: item.Share.Slug,
					Name: item.Share.Name,
				})
			}
		}
		if len(writeAccounts) > 0 {
			routes = append(routes, permissionRoute(paths, nil, writeAccounts, item.Share.Path, pathPrefix, hostname))
		}
		if len(readAccounts) == 0 {
			warnings = append(warnings, "share "+item.Share.Slug+" has no authorized users")
		}
	}
	if len(discoveryAccounts) > 0 {
		accounts := make([]account, 0, len(discoveryAccounts))
		usernames := make([]string, 0, len(discoveryAccounts))
		for username := range discoveryAccounts {
			usernames = append(usernames, username)
		}
		sort.Strings(usernames)
		for _, username := range usernames {
			accounts = append(accounts, discoveryAccounts[username])
			entries := discoveryEntries[username]
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].Slug == entries[j].Slug {
					return entries[i].Name < entries[j].Name
				}
				return entries[i].Slug < entries[j].Slug
			})
			discoveryEntries[username] = entries
		}
		rootPath := strings.TrimSuffix(input.ServerSettings.PublicBasePath, "/")
		if rootPath == "" {
			rootPath = "/"
		}
		rootPaths := []string{rootPath}
		if rootPath != "/" {
			rootPaths = append(rootPaths, rootPath+"/")
		}
		routes = append([]route{discoveryRoute(
			rootPaths,
			accounts,
			discoveryEntries,
			rootPath,
			hostname,
		)}, routes...)
	}
	listenPort := input.ServerSettings.HTTPPort
	configuration := config{Admin: adminConfig{Listen: "127.0.0.1:2019"}, Apps: appsConfig{HTTP: httpApp{HTTPPort: input.ServerSettings.HTTPPort, HTTPSPort: input.ServerSettings.HTTPSPort, Servers: map[string]httpServer{}}}}
	server := httpServer{Routes: routes}
	if input.TLSProfile != nil {
		listenPort = input.ServerSettings.HTTPSPort
		server.AutomaticHTTPS = &automaticHTTPSConfig{DisableRedirects: true}
		server.Routes = append(server.Routes, route{Match: []matcherSet{{Host: []string{hostname}}}, Handle: []any{staticResponseHandler{Handler: "static_response", StatusCode: 404}}, Terminal: true})
		tlsApp, policies, err := compileTLS(*input.TLSProfile, providers)
		if err != nil {
			return CompiledConfig{}, err
		}
		configuration.Apps.TLS = tlsApp
		if input.TLSProfile.Mode == domain.TLSModeInternal {
			configuration.Apps.PKI = &pkiApp{CertificateAuthorities: map[string]pkiCertificateAuthority{
				"local": {InstallTrust: false},
			}}
		}
		server.TLSConnectionPolicies = policies
	}
	server.Listen = []string{":" + strconv.Itoa(listenPort)}
	configuration.Apps.HTTP.Servers["davdeck"] = server
	if input.TLSProfile != nil {
		configuration.Apps.HTTP.Servers["davdeck-http-redirect"] = httpServer{
			Listen:         []string{":" + strconv.Itoa(input.ServerSettings.HTTPPort)},
			Routes:         []route{httpsRedirectRoute(hostname, input.ServerSettings.HTTPSPort)},
			AutomaticHTTPS: &automaticHTTPSConfig{Disabled: true},
		}
	}
	body, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return CompiledConfig{}, fmt.Errorf("encode Caddy JSON: %w", err)
	}
	body = append(body, '\n')
	hash := sha256.Sum256(body)
	return CompiledConfig{JSON: body, SHA256: hex.EncodeToString(hash[:]), Warnings: warnings}, nil
}

func joinPublicPath(basePath, child string) string {
	if basePath == "/" {
		return "/" + child
	}
	return strings.TrimSuffix(basePath, "/") + "/" + child
}

func compileAccounts(item ShareWithPermissions, users map[domain.ID]domain.User) ([]account, []account, error) {
	permissions := append([]domain.SharePermission(nil), item.Permissions...)
	sort.Slice(permissions, func(i, j int) bool { return permissions[i].UserID < permissions[j].UserID })
	seen := make(map[domain.ID]struct{}, len(permissions))
	read, write := make([]account, 0), make([]account, 0)
	for _, value := range permissions {
		if err := value.Validate(); err != nil {
			return nil, nil, fmt.Errorf("validate permission for share %s: %w", item.Share.ID, err)
		}
		if value.ShareID != item.Share.ID {
			return nil, nil, errors.New("permission references a different share")
		}
		if _, exists := seen[value.UserID]; exists {
			return nil, nil, fmt.Errorf("duplicate permission for user %s", value.UserID)
		}
		seen[value.UserID] = struct{}{}
		user, exists := users[value.UserID]
		if !exists {
			return nil, nil, fmt.Errorf("permission references unknown user %s", value.UserID)
		}
		if !user.Enabled || value.Permission == domain.PermissionNone {
			continue
		}
		entry := account{Username: user.Username, Password: user.PasswordHash}
		read = append(read, entry)
		if value.Permission == domain.PermissionReadWrite {
			write = append(write, entry)
		}
	}
	sort.Slice(read, func(i, j int) bool { return read[i].Username < read[j].Username })
	sort.Slice(write, func(i, j int) bool { return write[i].Username < write[j].Username })
	return read, write, nil
}

func permissionRoute(paths, methods []string, accounts []account, root, prefix, hostname string) route {
	matcher := matcherSet{Path: paths, Method: methods}
	if hostname != "" {
		matcher.Host = []string{hostname}
	}
	return route{Match: []matcherSet{matcher}, Handle: []any{
		authenticationHandler{Handler: "authentication", Providers: map[string]basicAuth{"http_basic": {Hash: hashAlgorithm{Algorithm: "bcrypt"}, Accounts: accounts}}},
		webDAVHandler{Handler: "webdav", Root: root, Prefix: prefix},
	}, Terminal: true}
}

func discoveryRoute(paths []string, accounts []account, entries map[string][]discoveryEntry, basePath, hostname string) route {
	matcher := matcherSet{Path: paths}
	if hostname != "" {
		matcher.Host = []string{hostname}
	}
	return route{Match: []matcherSet{matcher}, Handle: []any{
		authenticationHandler{Handler: "authentication", Providers: map[string]basicAuth{"http_basic": {Hash: hashAlgorithm{Algorithm: "bcrypt"}, Accounts: accounts}}},
		discoveryHandler{Handler: "davdeck_index", BasePath: basePath, Entries: entries},
	}, Terminal: true}
}

func httpsRedirectRoute(hostname string, httpsPort int) route {
	redirectHost := hostname
	if httpsPort != 443 {
		redirectHost += ":" + strconv.Itoa(httpsPort)
	}
	return route{
		Match: []matcherSet{{Host: []string{hostname}}},
		Handle: []any{staticResponseHandler{
			Handler:    "static_response",
			StatusCode: 308,
			Headers:    map[string][]string{"Location": {"https://" + redirectHost + "{http.request.uri}"}},
		}},
		Terminal: true,
	}
}

type config struct {
	Admin adminConfig `json:"admin"`
	Apps  appsConfig  `json:"apps"`
}
type adminConfig struct {
	Listen string `json:"listen"`
}
type appsConfig struct {
	HTTP httpApp `json:"http"`
	TLS  *tlsApp `json:"tls,omitempty"`
	PKI  *pkiApp `json:"pki,omitempty"`
}
type httpApp struct {
	HTTPPort  int                   `json:"http_port"`
	HTTPSPort int                   `json:"https_port"`
	Servers   map[string]httpServer `json:"servers"`
}
type httpServer struct {
	Listen                []string              `json:"listen"`
	Routes                []route               `json:"routes"`
	TLSConnectionPolicies []tlsConnectionPolicy `json:"tls_connection_policies,omitempty"`
	AutomaticHTTPS        *automaticHTTPSConfig `json:"automatic_https,omitempty"`
}
type automaticHTTPSConfig struct {
	Disabled         bool `json:"disable,omitempty"`
	DisableRedirects bool `json:"disable_redirects,omitempty"`
}
type route struct {
	Match    []matcherSet `json:"match"`
	Handle   []any        `json:"handle"`
	Terminal bool         `json:"terminal"`
}
type matcherSet struct {
	Path   []string `json:"path,omitempty"`
	Method []string `json:"method,omitempty"`
	Host   []string `json:"host,omitempty"`
}
type authenticationHandler struct {
	Handler   string               `json:"handler"`
	Providers map[string]basicAuth `json:"providers"`
}
type basicAuth struct {
	Hash     hashAlgorithm `json:"hash"`
	Accounts []account     `json:"accounts"`
}
type hashAlgorithm struct {
	Algorithm string `json:"algorithm"`
}
type account struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type webDAVHandler struct {
	Handler string `json:"handler"`
	Root    string `json:"root"`
	Prefix  string `json:"prefix"`
}

type discoveryEntry struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type discoveryHandler struct {
	Handler  string                      `json:"handler"`
	BasePath string                      `json:"base_path"`
	Entries  map[string][]discoveryEntry `json:"entries"`
}

type staticResponseHandler struct {
	Handler    string              `json:"handler"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
}

type tlsApp struct {
	Automation   *tlsAutomation   `json:"automation,omitempty"`
	Certificates *tlsCertificates `json:"certificates,omitempty"`
}

type pkiApp struct {
	CertificateAuthorities map[string]pkiCertificateAuthority `json:"certificate_authorities,omitempty"`
}

type pkiCertificateAuthority struct {
	InstallTrust bool `json:"install_trust"`
}

type tlsAutomation struct {
	Policies []tlsAutomationPolicy `json:"policies"`
}

type tlsAutomationPolicy struct {
	Subjects []string    `json:"subjects"`
	Issuers  []tlsIssuer `json:"issuers,omitempty"`
}

type tlsIssuer struct {
	Module     string          `json:"module"`
	Challenges *acmeChallenges `json:"challenges,omitempty"`
}

type acmeChallenges struct {
	DNS *dnsChallenge `json:"dns,omitempty"`
}

type dnsChallenge struct {
	Provider map[string]string `json:"provider"`
}

type tlsCertificates struct {
	LoadFiles []tlsFileLoader `json:"load_files"`
}

type tlsFileLoader struct {
	Certificate string   `json:"certificate"`
	Key         string   `json:"key"`
	Tags        []string `json:"tags"`
}

type tlsConnectionPolicy struct {
	Match                *tlsConnectionMatch  `json:"match,omitempty"`
	CertificateSelection *certificateSelector `json:"certificate_selection,omitempty"`
}

type tlsConnectionMatch struct {
	SNI []string `json:"sni"`
}

type certificateSelector struct {
	AnyTag []string `json:"any_tag"`
}

func compileTLS(profile domain.TLSProfile, providers map[domain.ID]domain.DNSProviderCredential) (*tlsApp, []tlsConnectionPolicy, error) {
	switch profile.Mode {
	case domain.TLSModeInternal:
		return &tlsApp{Automation: &tlsAutomation{Policies: []tlsAutomationPolicy{{Subjects: []string{profile.Hostname}, Issuers: []tlsIssuer{{Module: "internal"}}}}}}, nil, nil
	case domain.TLSModeCustom:
		policy := tlsConnectionPolicy{CertificateSelection: &certificateSelector{AnyTag: []string{"davdeck"}}}
		if net.ParseIP(profile.Hostname) == nil {
			policy.Match = &tlsConnectionMatch{SNI: []string{profile.Hostname}}
		}
		return &tlsApp{Certificates: &tlsCertificates{LoadFiles: []tlsFileLoader{{Certificate: profile.CertificatePath, Key: profile.PrivateKeyPath, Tags: []string{"davdeck"}}}}}, []tlsConnectionPolicy{policy}, nil
	case domain.TLSModeAutomatic:
		if profile.Challenge != domain.TLSChallengeDNS {
			return nil, nil, nil
		}
		if profile.DNSProviderID == nil {
			return nil, nil, errors.New("DNS TLS challenge has no provider")
		}
		credential, ok := providers[*profile.DNSProviderID]
		if !ok {
			return nil, nil, fmt.Errorf("DNS TLS challenge references unknown provider %s", *profile.DNSProviderID)
		}
		adapter, ok := dnsprovider.For(credential.Provider)
		if !ok {
			return nil, nil, fmt.Errorf("DNS provider %q is not supported by this build", credential.Provider)
		}
		issuer := tlsIssuer{Module: "acme", Challenges: &acmeChallenges{DNS: &dnsChallenge{Provider: adapter.CaddyProvider(credential.ID)}}}
		return &tlsApp{Automation: &tlsAutomation{Policies: []tlsAutomationPolicy{{Subjects: []string{profile.Hostname}, Issuers: []tlsIssuer{issuer}}}}}, nil, nil
	default:
		return nil, nil, nil
	}
}
