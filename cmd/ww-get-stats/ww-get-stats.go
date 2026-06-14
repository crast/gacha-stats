// This file is part of gacha-stats and is licensed under the GNU General
// Public License v3.0 (GPL-3.0). See LICENSE for details.
//
// Based on the import script by LuzeFiru:
// https://github.com/wuwatracker/wuwatracker/blob/main/import.ps1

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"time"
)

var conveneURLRegex = regexp.MustCompile(`https://aki-gm-resources(-oversea)?\.aki-game\.(net|com)/aki/gacha/index\.html#/record[^\s"]*`)

type logFile struct {
	path        string
	logType     string // "client" or "debug"
	installPath string
	modTime     time.Time
}

// multiflag allows a flag to be specified multiple times.
type multiflag []string

func (m *multiflag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiflag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	var extraPaths multiflag
	flag.Var(&extraPaths, "path", "additional game install path to search (can be specified multiple times)")
	flag.Parse()

	fmt.Println("Searching for Wuthering Waves log files...")

	seen := map[string]bool{}
	var searchPaths []string

	addPath := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if !seen[abs] {
			seen[abs] = true
			searchPaths = append(searchPaths, abs)
		}
	}

	for _, p := range extraPaths {
		addPath(p)
	}
	for _, p := range defaultSearchPaths() {
		addPath(p)
	}

	var collected []logFile
	for _, p := range searchPaths {
		fmt.Printf("Checking: %s\n", p)
		files := collectLogFiles(p)
		collected = append(collected, files...)
	}

	if len(collected) == 0 {
		fmt.Fprintln(os.Stderr, "\nNo log files found.")
		fmt.Fprintln(os.Stderr, "Tips:")
		fmt.Fprintln(os.Stderr, "  1. Open Convene History in-game first, then run this tool.")
		fmt.Fprintln(os.Stderr, "  2. Specify the game install path manually:")
		fmt.Fprintln(os.Stderr, "       ww-get-stats -path '/path/to/Wuthering Waves Game'")
		os.Exit(1)
	}

	// Sort by newest modification time.
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].modTime.After(collected[j].modTime)
	})

	fmt.Printf("\nFound %d log file(s), checking newest first:\n", len(collected))
	for _, lf := range collected {
		fmt.Printf("  [%s] %s\n", lf.modTime.Format("2006-01-02 15:04:05"), lf.path)
	}
	fmt.Println()

	url, foundIn := findURL(collected)
	if url == "" {
		fmt.Fprintln(os.Stderr, "No convene URL found in any log file.")
		fmt.Fprintln(os.Stderr, "Make sure you have opened Convene History in-game before running this tool.")
		os.Exit(1)
	}

	fmt.Printf("URL found in: %s\n", foundIn)
	fmt.Printf("\nConvene Record URL:\n%s\n\n", url)

	if err := copyToClipboard(url); err != nil {
		fmt.Fprintf(os.Stderr, "Note: could not copy to clipboard (%v). Copy the URL above manually.\n", err)
	} else {
		fmt.Println("URL copied to clipboard. Paste it at wuwatracker.com and click Import History.")
	}
}

// defaultSearchPaths returns the standard Steam install locations for the current OS.
func defaultSearchPaths() []string {
	var bases []string

	switch runtime.GOOS {
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			break
		}
		bases = []string{
			filepath.Join(home, ".local/share/Steam/steamapps/common/Wuthering Waves"),
			filepath.Join(home, ".steam/steam/steamapps/common/Wuthering Waves"),
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			break
		}
		bases = []string{
			filepath.Join(home, "Library/Application Support/Steam/steamapps/common/Wuthering Waves"),
		}
	case "windows":
		bases = []string{
			`C:\Program Files (x86)\Steam\steamapps\common\Wuthering Waves`,
			`C:\Steam\steamapps\common\Wuthering Waves`,
		}
	}

	// Some installs place files directly under the base; others use a "Wuthering Waves Game" subdirectory.
	var paths []string
	seen := map[string]bool{}
	for _, b := range bases {
		for _, p := range []string{b, filepath.Join(b, "Wuthering Waves Game")} {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// collectLogFiles checks whether installPath is a valid game directory and returns any log files found.
func collectLogFiles(installPath string) []logFile {
	info, err := os.Stat(installPath)
	if err != nil || !info.IsDir() {
		return nil
	}

	checkEngineIni(installPath)

	var files []logFile

	clientLog := filepath.Join(installPath, "Client", "Saved", "Logs", "Client.log")
	if fi, err := os.Stat(clientLog); err == nil {
		fmt.Printf("  Queued Client.log (modified: %s)\n", fi.ModTime().Format("2006-01-02 15:04:05"))
		files = append(files, logFile{
			path:        clientLog,
			logType:     "client",
			installPath: installPath,
			modTime:     fi.ModTime(),
		})
	}

	// This path retains "Win64" in the name even on Linux/Proton since it's just a folder name.
	debugLog := filepath.Join(installPath, "Client", "Binaries", "Win64", "ThirdParty",
		"KrPcSdk_Global", "KRSDKRes", "KRSDKWebView", "debug.log")
	if fi, err := os.Stat(debugLog); err == nil {
		fmt.Printf("  Queued debug.log    (modified: %s)\n", fi.ModTime().Format("2006-01-02 15:04:05"))
		files = append(files, logFile{
			path:        debugLog,
			logType:     "debug",
			installPath: installPath,
			modTime:     fi.ModTime(),
		})
	}

	return files
}

// checkEngineIni warns if logging has been disabled in Engine.ini.
func checkEngineIni(installPath string) {
	iniPath := filepath.Join(installPath, "Client", "Saved", "Config", "WindowsNoEditor", "Engine.ini")
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return
	}
	matched, _ := regexp.MatchString(`(?i)\[Core\.Log\][\r\n]+Global=(off|none)`, string(data))
	if matched {
		fmt.Fprintf(os.Stderr, "\nWARNING: %s\n", iniPath)
		fmt.Fprintln(os.Stderr, "  Engine.ini has logging disabled ([Core.Log] Global=off/none).")
		fmt.Fprintln(os.Stderr, "  Remove or comment out that section, then restart the game and open Convene History.\n")
	}
}

// findURL iterates log files (expected to be sorted newest-first) and returns the first convene URL found.
// When a debug.log is the trigger, we prefer its sibling Client.log from the same install.
func findURL(logs []logFile) (url, foundIn string) {
	for _, lf := range logs {
		if lf.logType == "debug" {
			// Try the Client.log from the same installation first.
			for _, other := range logs {
				if other.logType == "client" && other.installPath == lf.installPath {
					if u := extractURLFromLog(other); u != "" {
						return u, other.path
					}
					break
				}
			}
		}
		if u := extractURLFromLog(lf); u != "" {
			return u, lf.path
		}
	}
	return "", ""
}

// extractURLFromLog reads a log file and returns the last matching convene URL.
func extractURLFromLog(lf logFile) string {
	data, err := readFile(lf.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", lf.path, err)
		return ""
	}

	if lf.logType == "client" {
		// Try XOR-decrypted content first (current game versions encrypt Client.log).
		if u := extractURL(string(decryptClientLog(data))); u != "" {
			return u
		}
		// Fall back to raw bytes in case an older unencrypted version is present.
		return extractURL(string(data))
	}

	// debug.log is plain UTF-8 text.
	return extractURL(string(data))
}

// decryptClientLog applies the XOR cipher Kuro uses on Client.log.
// Discovered by @kyuxu, shared by @RabbyDevs.
func decryptClientLog(data []byte) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		if (b&0x0F)%2 == 1 {
			out[i] = b ^ 0xA5
		} else {
			out[i] = b ^ 0xEF
		}
	}
	return out
}

// extractURL returns the last convene URL in content, or "" if none found.
func extractURL(content string) string {
	matches := conveneURLRegex.FindAllString(content, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

// readFile reads a file with shared access (on Linux all files are readable regardless of other processes).
func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// copyToClipboard tries platform-appropriate clipboard tools in order.
func copyToClipboard(text string) error {
	switch runtime.GOOS {
	case "linux":
		// Try wl-copy (Wayland) then xclip then xsel.
		for _, args := range [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		} {
			if err := pipeToCmd(text, args[0], args[1:]...); err == nil {
				return nil
			}
		}
		return fmt.Errorf("no clipboard tool found (install wl-clipboard, xclip, or xsel)")
	case "darwin":
		return pipeToCmd(text, "pbcopy")
	case "windows":
		return pipeToCmd(text, "clip")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

func pipeToCmd(input, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := fmt.Fprint(stdin, input); err != nil {
		return err
	}
	stdin.Close()
	return cmd.Wait()
}
