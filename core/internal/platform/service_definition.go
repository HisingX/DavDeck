package platform

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

func renderLaunchdDefinition(config ServiceConfig) ([]byte, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	arguments := append([]string{config.Executable}, config.Arguments...)
	var values strings.Builder
	for _, argument := range arguments {
		var encoded bytes.Buffer
		if err := xml.EscapeText(&encoded, []byte(argument)); err != nil {
			return nil, err
		}
		fmt.Fprintf(&values, "    <string>%s</string>\n", encoded.String())
	}
	user := ""
	if config.User != "" {
		var encoded bytes.Buffer
		if err := xml.EscapeText(&encoded, []byte(config.User)); err != nil {
			return nil, err
		}
		user = "  <key>UserName</key>\n  <string>" + encoded.String() + "</string>\n"
	}
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>dev.davdeck.davd</string>
  <key>ProgramArguments</key>
  <array>
%s  </array>
%s  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict><key>SuccessfulExit</key><false/></dict>
</dict>
</plist>
`, values.String(), user)
	return []byte(body), nil
}

func renderSystemdDefinition(config ServiceConfig) ([]byte, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	arguments := append([]string{config.Executable}, config.Arguments...)
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quoted = append(quoted, quoteSystemdArgument(argument))
	}
	description := config.Description
	if description == "" {
		description = serviceDisplayName
	}
	user := ""
	if config.User != "" {
		user = "User=" + config.User + "\n"
	}
	body := fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
%sExecStart=%s
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, description, user, strings.Join(quoted, " "))
	return []byte(body), nil
}

func quoteSystemdArgument(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, `%`, `%%`)
	return `"` + value + `"`
}
