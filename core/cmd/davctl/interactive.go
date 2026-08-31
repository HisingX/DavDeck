package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"davdeck.dev/davdeck/core/internal/buildinfo"
	"davdeck.dev/davdeck/core/internal/client"
)

// runInteractive is deliberately a line-oriented menu rather than a full
// screen TUI. It works over SSH, keeps output readable in terminal scrollback,
// and delegates every operation to the existing command/API implementations.
func runInteractive(deps dependencies, apiClient managementClient) int {
	session := &interactiveSession{
		deps: deps,
		api:  apiClient,
		in:   bufio.NewReader(deps.stdin),
	}
	session.maybeFirstRun()

	fmt.Fprintf(deps.stdout, "\nDavDeck %s\n", currentVersion())
	fmt.Fprintln(deps.stdout, "────────────────────────────────────")
	for {
		choice, err := session.menu("Main menu", []string{
			"Server Status",
			"Users",
			"Shares",
			"Permissions",
			"HTTPS / TLS",
			"Configuration",
			"Logs",
			"Diagnostics",
			"Backup / Restore",
			"Service",
			"Exit",
		})
		if err == io.EOF || choice == 11 {
			return exitSuccess
		}
		if err != nil {
			return printFailure(deps, false, exitOperational, "INTERACTIVE_INPUT_FAILED", "Unable to read interactive input")
		}
		switch choice {
		case 1:
			runStatus(deps, false, apiClient)
		case 2:
			session.users()
		case 3:
			session.shares()
		case 4:
			session.permissions()
		case 5:
			session.tls()
		case 6:
			session.configuration()
		case 7:
			session.logs()
		case 8:
			runDoctor(deps, false, apiClient)
		case 9:
			session.backup()
		case 10:
			session.service()
		}
	}
}

type interactiveSession struct {
	deps dependencies
	api  managementClient
	in   *bufio.Reader
}

func currentVersion() string {
	return buildinfo.Current().Version
}

func (s *interactiveSession) menu(title string, entries []string) (int, error) {
	fmt.Fprintf(s.deps.stdout, "\n%s\n", title)
	for index, entry := range entries {
		fmt.Fprintf(s.deps.stdout, "  %d) %s\n", index+1, entry)
	}
	fmt.Fprintln(s.deps.stdout, "  b) Back / q) Quit")
	for {
		value, err := s.readLine("> ")
		if err != nil {
			return 0, err
		}
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "b" {
			return 0, nil
		}
		if lower == "q" || lower == "quit" || lower == "exit" {
			return len(entries), nil
		}
		choice, err := strconv.Atoi(lower)
		if err == nil && choice >= 1 && choice <= len(entries) {
			return choice, nil
		}
		fmt.Fprintln(s.deps.stdout, "Choose a menu number, b to go back, or q to quit.")
	}
}

func (s *interactiveSession) readLine(prompt string) (string, error) {
	fmt.Fprint(s.deps.stdout, prompt)
	value, err := s.in.ReadString('\n')
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r")), err
}

func (s *interactiveSession) prompt(label string) (string, bool) {
	value, err := s.readLine(label)
	if err != nil {
		if err == io.EOF && value != "" {
			return value, true
		}
		return "", false
	}
	return value, true
}

func (s *interactiveSession) confirm(prompt string) bool {
	value, ok := s.prompt(prompt + " [y/N] ")
	if !ok {
		return false
	}
	switch strings.ToLower(value) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func (s *interactiveSession) users() {
	for {
		choice, err := s.menu("Users", []string{"List", "Add", "Change Password", "Enable / Disable", "Delete", "Back"})
		if err != nil || choice == 0 || choice == 6 {
			return
		}
		switch choice {
		case 1:
			runUser(s.deps, false, s.api, []string{"list"})
		case 2:
			s.addUser()
		case 3:
			s.changePassword()
		case 4:
			s.toggleUser()
		case 5:
			s.deleteUser()
		}
	}
}

func (s *interactiveSession) selectUser() (client.User, bool) {
	users, err := s.api.ListUsers(context.Background())
	if err != nil {
		printClientError(s.deps, false, err)
		return client.User{}, false
	}
	if len(users) == 0 {
		fmt.Fprintln(s.deps.stdout, "No users.")
		return client.User{}, false
	}
	for index, user := range users {
		state := "enabled"
		if !user.Enabled {
			state = "disabled"
		}
		fmt.Fprintf(s.deps.stdout, "  %d) %s (%s)\n", index+1, user.Username, state)
	}
	value, ok := s.prompt("Select user (b to cancel): ")
	if !ok || strings.EqualFold(value, "b") {
		return client.User{}, false
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 1 || index > len(users) {
		fmt.Fprintln(s.deps.stdout, "Invalid user selection.")
		return client.User{}, false
	}
	return users[index-1], true
}

func (s *interactiveSession) addUser() {
	username, ok := s.prompt("Username: ")
	if !ok || username == "" {
		return
	}
	password, ok := s.secretPair("Password: ", "Confirm:  ")
	if !ok {
		return
	}
	s.runUserWithPassword([]string{"add", username}, password)
}

func (s *interactiveSession) changePassword() {
	user, ok := s.selectUser()
	if !ok {
		return
	}
	password, ok := s.secretPair("New password: ", "Confirm:     ")
	if !ok {
		return
	}
	s.runUserWithPassword([]string{"passwd", string(user.ID)}, password)
}

func (s *interactiveSession) toggleUser() {
	user, ok := s.selectUser()
	if !ok {
		return
	}
	action := "disable"
	if !user.Enabled {
		action = "enable"
	}
	if s.confirm(fmt.Sprintf("%s user %q?", strings.Title(action), user.Username)) {
		runUser(s.deps, false, s.api, []string{action, string(user.ID)})
	}
}

func (s *interactiveSession) deleteUser() {
	user, ok := s.selectUser()
	if !ok || !s.confirm(fmt.Sprintf("Delete user %q? Files are preserved.", user.Username)) {
		return
	}
	runUser(s.deps, false, s.api, []string{"delete", string(user.ID)})
}

func (s *interactiveSession) secretPair(firstPrompt, secondPrompt string) (string, bool) {
	first, code := obtainPassword(s.deps, false, false, firstPrompt)
	if code != exitSuccess {
		return "", false
	}
	second, code := obtainPassword(s.deps, false, false, secondPrompt)
	if code != exitSuccess {
		return "", false
	}
	if first != second {
		fmt.Fprintln(s.deps.stdout, "Passwords do not match.")
		return "", false
	}
	return first, true
}

func (s *interactiveSession) runUserWithPassword(arguments []string, password string) {
	deps := s.deps
	deps.stdin = strings.NewReader(password + "\n")
	runUser(deps, false, s.api, append(arguments, "--password-stdin"))
}

func (s *interactiveSession) shares() {
	for {
		choice, err := s.menu("Shares", []string{"List", "Add", "Edit", "Enable / Disable", "Delete", "Back"})
		if err != nil || choice == 0 || choice == 6 {
			return
		}
		switch choice {
		case 1:
			runShare(s.deps, false, s.api, []string{"list"})
		case 2:
			s.addShare()
		case 3:
			s.editShare()
		case 4:
			s.toggleShare()
		case 5:
			s.deleteShare()
		}
	}
}

func (s *interactiveSession) selectShare() (client.Share, bool) {
	shares, err := s.api.ListShares(context.Background())
	if err != nil {
		printClientError(s.deps, false, err)
		return client.Share{}, false
	}
	if len(shares) == 0 {
		fmt.Fprintln(s.deps.stdout, "No shares.")
		return client.Share{}, false
	}
	for index, share := range shares {
		state := "enabled"
		if !share.Enabled {
			state = "disabled"
		}
		fmt.Fprintf(s.deps.stdout, "  %d) %s (%s, %s)\n", index+1, share.Name, share.Slug, state)
	}
	value, ok := s.prompt("Select share (b to cancel): ")
	if !ok || strings.EqualFold(value, "b") {
		return client.Share{}, false
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 1 || index > len(shares) {
		fmt.Fprintln(s.deps.stdout, "Invalid share selection.")
		return client.Share{}, false
	}
	return shares[index-1], true
}

func (s *interactiveSession) addShare() {
	name, ok := s.prompt("Name: ")
	if !ok || name == "" {
		return
	}
	path, ok := s.prompt("Path: ")
	if !ok || path == "" {
		return
	}
	slug, ok := s.prompt("Slug (blank to derive): ")
	if !ok {
		return
	}
	arguments := []string{"add", name, path}
	if slug != "" {
		arguments = append(arguments, "--slug", slug)
	}
	runShare(s.deps, false, s.api, arguments)
}

func (s *interactiveSession) editShare() {
	share, ok := s.selectShare()
	if !ok {
		return
	}
	arguments := []string{"update", string(share.ID)}
	for _, field := range []struct{ flag, label string }{{"--name", "Name"}, {"--slug", "Slug"}, {"--path", "Path"}} {
		value, inputOK := s.prompt(fmt.Sprintf("%s (blank to keep %q): ", field.label, fieldValue(share, field.flag)))
		if !inputOK {
			return
		}
		if value != "" {
			arguments = append(arguments, field.flag, value)
		}
	}
	if len(arguments) == 2 {
		fmt.Fprintln(s.deps.stdout, "No changes requested.")
		return
	}
	runShare(s.deps, false, s.api, arguments)
}

func fieldValue(share client.Share, flag string) string {
	switch flag {
	case "--name":
		return share.Name
	case "--slug":
		return share.Slug
	case "--path":
		return share.Path
	default:
		return ""
	}
}

func (s *interactiveSession) toggleShare() {
	share, ok := s.selectShare()
	if !ok {
		return
	}
	flag := "--disable"
	action := "Disable"
	if !share.Enabled {
		flag = "--enable"
		action = "Enable"
	}
	if s.confirm(fmt.Sprintf("%s share %q?", action, share.Name)) {
		runShare(s.deps, false, s.api, []string{"update", string(share.ID), flag})
	}
}

func (s *interactiveSession) deleteShare() {
	share, ok := s.selectShare()
	if !ok || !s.confirm(fmt.Sprintf("Delete share %q? Physical files are preserved.", share.Name)) {
		return
	}
	runShare(s.deps, false, s.api, []string{"remove", string(share.ID)})
}

func (s *interactiveSession) permissions() {
	for {
		choice, err := s.menu("Permissions", []string{"List", "Set Permission", "Back"})
		if err != nil || choice == 0 || choice == 3 {
			return
		}
		switch choice {
		case 1:
			runACL(s.deps, false, s.api, []string{"list"})
		case 2:
			s.setPermission()
		}
	}
}

func (s *interactiveSession) setPermission() {
	user, ok := s.selectUser()
	if !ok {
		return
	}
	share, ok := s.selectShare()
	if !ok {
		return
	}
	choice, ok := s.prompt("Permission (1 read/write, 2 read only, 3 none): ")
	if !ok {
		return
	}
	permission := map[string]string{"1": "read-write", "2": "read", "3": "none"}[choice]
	if permission == "" {
		fmt.Fprintln(s.deps.stdout, "Invalid permission.")
		return
	}
	runACL(s.deps, false, s.api, []string{"set", string(share.ID), string(user.ID), permission})
}

func (s *interactiveSession) tls() {
	for {
		choice, err := s.menu("HTTPS / TLS", []string{"Current status", "HTTP Only", "Internal HTTPS", "Automatic HTTPS", "Custom Certificate", "Back"})
		if err != nil || choice == 0 || choice == 6 {
			return
		}
		switch choice {
		case 1:
			runTLS(s.deps, false, s.api, []string{"show"})
		case 2:
			if s.confirm("Disable HTTPS?") {
				runTLS(s.deps, false, s.api, []string{"disable"})
			}
		case 3:
			s.configureTLS("internal")
		case 4:
			s.configureTLS("automatic")
		case 5:
			s.configureCustomTLS()
		}
	}
}

func (s *interactiveSession) configureTLS(mode string) {
	hostname, ok := s.prompt("Hostname: ")
	if ok && hostname != "" {
		runTLS(s.deps, false, s.api, []string{mode, hostname})
	}
}

func (s *interactiveSession) configureCustomTLS() {
	hostname, ok := s.prompt("Hostname: ")
	if !ok || hostname == "" {
		return
	}
	certificate, ok := s.prompt("Certificate path: ")
	if !ok || certificate == "" {
		return
	}
	privateKey, ok := s.prompt("Private key path: ")
	if !ok || privateKey == "" {
		return
	}
	runTLS(s.deps, false, s.api, []string{"custom", "--hostname", hostname, "--cert", certificate, "--key", privateKey})
}

func (s *interactiveSession) configuration() {
	for {
		choice, err := s.menu("Configuration", []string{"Status", "Validate", "Apply", "Revisions", "Back"})
		if err != nil || choice == 0 || choice == 5 {
			return
		}
		switch choice {
		case 1:
			runConfig(s.deps, false, s.api, []string{"status"})
		case 2:
			runConfig(s.deps, false, s.api, []string{"validate"})
		case 3:
			runConfig(s.deps, false, s.api, []string{"apply"})
		case 4:
			runRevision(s.deps, false, s.api, []string{"list"})
		}
	}
}

func (s *interactiveSession) logs() {
	for {
		choice, err := s.menu("Logs", []string{"Daemon", "Caddy", "Errors", "Back"})
		if err != nil || choice == 0 || choice == 4 {
			return
		}
		switch choice {
		case 1:
			runLogs(s.deps, false, s.api, []string{"--component", "davd", "--limit", "50"})
		case 2:
			runLogs(s.deps, false, s.api, []string{"--component", "caddy", "--limit", "50"})
		case 3:
			runLogs(s.deps, false, s.api, []string{"--level", "ERROR", "--limit", "50"})
		}
	}
}

func (s *interactiveSession) backup() {
	for {
		choice, err := s.menu("Backup / Restore", []string{"Export configuration", "Import configuration", "Back"})
		if err != nil || choice == 0 || choice == 3 {
			return
		}
		switch choice {
		case 1:
			if path, ok := s.prompt("Output path: "); ok && path != "" {
				runConfig(s.deps, false, s.api, []string{"export", "--output", path})
			}
		case 2:
			if path, ok := s.prompt("YAML file path: "); ok && path != "" && s.confirm("Import this configuration?") {
				runConfig(s.deps, false, s.api, []string{"import", path})
			}
		}
	}
}

func (s *interactiveSession) service() {
	for {
		choice, err := s.menu("Service", []string{"Status", "Install", "Start", "Stop", "Uninstall", "Back"})
		if err != nil || choice == 0 || choice == 6 {
			return
		}
		switch choice {
		case 1:
			runService(s.deps, false, s.api, []string{"status"})
		case 2:
			if s.confirm("Install the system service?") {
				runService(s.deps, false, s.api, []string{"install"})
			}
		case 3:
			runService(s.deps, false, s.api, []string{"start"})
		case 4:
			runService(s.deps, false, s.api, []string{"stop"})
		case 5:
			if s.confirm("Uninstall the system service? Configuration and data are preserved.") {
				runService(s.deps, false, s.api, []string{"uninstall"})
			}
		}
	}
}

func (s *interactiveSession) maybeFirstRun() {
	users, userErr := s.api.ListUsers(context.Background())
	shares, shareErr := s.api.ListShares(context.Background())
	if userErr != nil || shareErr != nil || len(users) != 0 || len(shares) != 0 {
		return
	}
	fmt.Fprintln(s.deps.stdout, "\nWelcome to DavDeck.")
	fmt.Fprintln(s.deps.stdout, "This appears to be a new installation.")
	if s.confirm("Run initial setup?") {
		s.firstRunWizard()
	}
}

func (s *interactiveSession) firstRunWizard() {
	fmt.Fprintln(s.deps.stdout, "\nInitial setup")
	settings, err := s.api.ServerSettings(context.Background())
	if err == nil {
		httpPort := s.wizardValue(fmt.Sprintf("HTTP port [%d]: ", settings.HTTPPort), strconv.Itoa(settings.HTTPPort))
		httpsPort := s.wizardValue(fmt.Sprintf("HTTPS port [%d]: ", settings.HTTPSPort), strconv.Itoa(settings.HTTPSPort))
		runServer(s.deps, false, s.api, []string{"ports", "--http", httpPort, "--https", httpsPort})
	}

	fmt.Fprintln(s.deps.stdout, "TLS mode: 1) HTTP only  2) Internal HTTPS  3) Automatic HTTPS  4) Custom")
	mode, ok := s.prompt("Select TLS mode [1]: ")
	if !ok || mode == "" {
		mode = "1"
	}
	switch mode {
	case "2":
		s.configureTLS("internal")
	case "3":
		s.configureTLS("automatic")
	case "4":
		s.configureCustomTLS()
	default:
		runTLS(s.deps, false, s.api, []string{"disable"})
	}

	username, ok := s.prompt("First username: ")
	if !ok || username == "" {
		return
	}
	password, ok := s.secretPair("Password: ", "Confirm:  ")
	if !ok {
		return
	}
	s.runUserWithPassword([]string{"add", username}, password)
	name, ok := s.prompt("First share name: ")
	if !ok || name == "" {
		return
	}
	path, ok := s.prompt("First share path: ")
	if !ok || path == "" {
		return
	}
	s.runCommandShare([]string{"add", name, path})

	users, userErr := s.api.ListUsers(context.Background())
	shares, shareErr := s.api.ListShares(context.Background())
	if userErr != nil || shareErr != nil || len(users) == 0 || len(shares) == 0 {
		return
	}
	fmt.Fprintln(s.deps.stdout, "Permission: 1) Read / Write  2) Read Only  3) None")
	permission, ok := s.prompt("Select permission [1]: ")
	if !ok || permission == "" {
		permission = "1"
	}
	permissionValue := map[string]string{"1": "read-write", "2": "read", "3": "none"}[permission]
	if permissionValue == "" {
		permissionValue = "read-write"
	}
	runACL(s.deps, false, s.api, []string{"set", string(shares[0].ID), string(users[0].ID), permissionValue})
	fmt.Fprintln(s.deps.stdout, "Reviewing configuration...")
	runConfig(s.deps, false, s.api, []string{"status"})
	if s.confirm("Apply this configuration?") {
		runConfig(s.deps, false, s.api, []string{"apply"})
	}
}

func (s *interactiveSession) wizardValue(prompt, fallback string) string {
	value, ok := s.prompt(prompt)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func (s *interactiveSession) runCommandShare(arguments []string) {
	runShare(s.deps, false, s.api, arguments)
}
