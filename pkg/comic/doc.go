// Package comic provides the foundational types and algorithms for the
// go-comic-converter pipeline. It is designed to be imported by both the
// CLI tool and external Go programs.
//
// The package contains format-agnostic algorithms (part splitting, viewport
// computation) and registry interfaces that output writers and source loaders
// can implement. The concrete implementations live in subpackages.
package comic
