//go:build !linux

package platform

func systemPaths() Paths { return Paths{} }
