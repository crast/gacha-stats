// This file is part of gacha-stats and is licensed under the GNU General
// Public License v3.0 (GPL-3.0). See LICENSE for details.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.design/x/clipboard"
)

var cache = map[string]cacheEntry{}
var desiredUID string
var pathFlag string

func main() {
	flag.StringVar(&desiredUID, "uid", "", "User ID to prefer")
	flag.StringVar(&pathFlag, "path", "", "Path to Genshin Impact game directory")
	flag.Parse()
	if err := runIt(); err != nil {
		log.Fatal(err)
	}
}

type cacheEntry struct {
	Worked bool
	Date   time.Time `json:"t"`
}

func runIt() error {

	err := clipboard.Init()
	if err != nil {
		return err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	// get cache
	cacheFile := filepath.Join(cacheDir, "gi-stats.json")
	buf, err := os.ReadFile(cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(buf) != 0 {
		err = json.Unmarshal(buf, &cache)
		if err != nil {
			return err
		}
	}

	name, err := findCacheFromCandidates()
	if err != nil {
		return err
	}

	ch := make(chan string, 10)
	f, _ := os.Open(name)
	go func() {
		defer f.Close()
		findStrings(f, ch, true)
	}()

	var allFoundStrings []string
	for s := range ch {
		allFoundStrings = append(allFoundStrings, s)
	}

	cacheSkipped := 0
	uidSkipped := 0
	goodURI := ""
	userID := ""
	var tryLater []string
	for _, s := range slices.Backward(allFoundStrings) {
		if !strings.Contains(s, "GachaLog") || len(s) < 10 {
			continue
		}
		parts := strings.SplitN(s, "/", 3)
		if len(parts) < 3 {
			continue
		}
		uri := parts[2]
		entry, ok := cache[uri]
		if ok {
			if entry.Worked {
				tryLater = append(tryLater, uri)
			}
			cacheSkipped += 1
			continue
		}

		fmt.Println(parts[2])
		checkRes, err := checkURIGenshin(parts[2])
		if err != nil {
			return err
		}
		if desiredUID != "" && checkRes.UID != desiredUID {
			uidSkipped += 1
			fmt.Printf("Skipping, UID %v != %v", checkRes.UID, desiredUID)
			continue
		}
		cache[uri] = cacheEntry{
			Worked: checkRes.Good,
			Date:   time.Now().Round(time.Second).UTC(),
		}
		if checkRes.Good {
			userID = checkRes.UID
			goodURI = uri
			break
		} else {
			fmt.Printf("FAIL URI: (reason %s) %s\n", checkRes.Reason, goodURI)
		}
	}
	if userID != "" {
		fmt.Printf("genshin UID: %v\n", userID)
	}
	if goodURI != "" {
		fmt.Printf("Good URI: \n%s\n", goodURI)
		clipboard.Write(clipboard.FmtText, []byte(goodURI))
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			cmd := exec.CommandContext(context.Background(), "wl-copy", "--type", "text/plain", goodURI)
			cmd.Run()
		}
	}
	buf, err = json.Marshal(cache)
	if err != nil {
		return err
	}
	os.WriteFile(cacheFile, buf, 0666)
	if cacheSkipped > 0 {
		fmt.Printf("%d skipped due to cache\n", cacheSkipped)
	}
	if uidSkipped > 0 {
		fmt.Printf("%d skipped due to UID\n", uidSkipped)
	}
	return nil
}

func candidateWebCachePaths() []string {
	home, _ := os.UserHomeDir()
	const suffix = "GenshinImpact_Data/webCaches"
	var candidates []string

	if pathFlag != "" {
		candidates = append(candidates, filepath.Join(pathFlag, suffix))
	}

	// twintail launcher — uuid subdir is unique per install, so list it
	twintailBase := filepath.Join(home, ".local/share/twintaillauncher/games/hk4e_global")
	if entries, err := os.ReadDir(twintailBase); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, filepath.Join(twintailBase, e.Name(), suffix))
			}
		}
	}

	// steam
	for _, steamLib := range []string{
		filepath.Join(home, ".local/share/Steam/steamapps/common/Genshin Impact"),
		filepath.Join(home, ".steam/steam/steamapps/common/Genshin Impact"),
	} {
		candidates = append(candidates, filepath.Join(steamLib, suffix))
	}

	// ~/Games
	candidates = append(candidates, filepath.Join(home, "Games/Genshin Impact", suffix))

	return candidates
}

func findCacheFromCandidates() (string, error) {
	candidates := candidateWebCachePaths()
	for _, base := range candidates {
		path, err := findGreatestMatchingCache(base)
		if err == nil {
			fmt.Printf("using cache base: %s\n", base)
			return path, nil
		}
	}
	return "", fmt.Errorf("could not find Genshin Impact webCaches in any known location; use -path to specify the game directory")
}

func findGreatestMatchingCache(base string) (string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("could not read dir %s: %w", base, err)
	}
	biggest := ""
	for _, e := range entries {
		if e.Name() > biggest {
			biggest = e.Name()
		}
	}
	if biggest == "" {
		return "", fmt.Errorf("no entries in %s", base)
	}
	fmt.Printf("biggest: %s\n", biggest)
	return filepath.Join(base, biggest, "Cache/Cache_Data/data_2"), nil
}

var maybeErrorStrings = [][]byte{
	[]byte("timeout"),
	[]byte("time out"),
	[]byte("authkey error"),
}

type URICheckResult struct {
	Good   bool
	Reason string
	Body   []byte

	// post-parsing
	UID string
}

func checkURIGenshin(uri string) (URICheckResult, error) {
	r, err := checkURI(uri)
	if err != nil || !r.Good {
		return r, err
	}

	var decoded genshinResp
	jsonErr := json.Unmarshal(r.Body, &decoded)
	if jsonErr != nil {
		return r, fmt.Errorf("json err %w original err %w", jsonErr, err)
	}
	if decoded.Data != nil && len(decoded.Data.List) > 0 {
		r.UID = decoded.Data.List[0].Uid
	}

	return r, err
}

func checkURI(uri string) (r URICheckResult, err error) {
	resp, err := http.Get(uri)
	if err != nil {
		r.Reason = "HTTP error"
		return r, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		r.Reason = fmt.Sprintf("HTTP status code %d", resp.StatusCode)
		return r, err
	}
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	r.Body = buf.Bytes()

	for _, errString := range maybeErrorStrings {
		if bytes.Contains(r.Body, errString) {
			r.Reason = string(errString)
			return r, nil
		}
	}
	r.Good = true
	return
}

const minLen = 10
const maxLen = 4096

func findStrings(file *os.File, ch chan<- string, ascii bool) {
	in := bufio.NewReader(file)
	str := make([]rune, 0, maxLen)
	filePos := int64(0)
	send := func() {
		if len(str) >= minLen {
			s := string(str)
			ch <- s
		}
		str = str[0:0]
	}
	for {
		var (
			r   rune
			wid int
			err error
		)
		// One string per loop.
		for ; ; filePos += int64(wid) {
			r, wid, err = in.ReadRune()
			if err != nil {
				if err != io.EOF {
					log.Print(err)
				}
				close(ch)
				return
			}
			if !strconv.IsPrint(r) || ascii && r >= 0xFF {
				send()
				continue
			}
			// It's printable. Keep it.
			if len(str) >= maxLen {
				send()
			}
			str = append(str, r)
		}
	}
}

type genshinResp struct {
	Retcode int `json:"retcode"`

	Data *genshinData `json:"data"`
}

type genshinData struct {
	List []genshinListEntry `json:"list"`
}
type genshinListEntry struct {
	Uid       string
	GachaType string
}
