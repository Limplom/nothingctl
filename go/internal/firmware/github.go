// Package firmware provides GitHub API helpers, firmware download, extraction,
// and resolution for Nothing Phone devices.
package firmware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const (
	githubAPIBase = "https://api.github.com/repos/spike0en/nothing_archive"
	userAgent     = "nothing-firmware-manager/2.0"
)

// GhGet performs an HTTP GET against the nothing_archive GitHub API and returns
// the raw response body. It handles rate limiting by returning an error with
// the rate-limit reset time when a 403/429 is received.
func GhGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nterrors.FirmwareError("building request: " + err.Error())
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nterrors.FirmwareError("HTTP GET failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		reset := resp.Header.Get("X-RateLimit-Reset")
		return nil, nterrors.FirmwareError(
			fmt.Sprintf("GitHub API rate limited (HTTP %d). Reset at unix timestamp: %s", resp.StatusCode, reset),
		)
	}
	if resp.StatusCode != 200 {
		return nil, nterrors.FirmwareError(
			fmt.Sprintf("GitHub API returned HTTP %d for %s", resp.StatusCode, url),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nterrors.FirmwareError("reading response body: " + err.Error())
	}
	return body, nil
}

// FetchReleases fetches up to 50 GitHub releases for owner/repo and returns
// them as a slice of raw JSON objects.
func FetchReleases(owner, repo string) ([]map[string]any, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=50", owner, repo)
	body, err := GhGet(url)
	if err != nil {
		return nil, err
	}
	var releases []map[string]any
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, nterrors.FirmwareError("parsing releases JSON: " + err.Error())
	}
	return releases, nil
}

// LatestRelease returns the most recent release from a list, keyed by the
// 6-digit date segment embedded in Nothing Archive tag names (YYMMDD).
func LatestRelease(owner, repo string) (map[string]any, error) {
	releases, err := FetchReleases(owner, repo)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, nterrors.FirmwareError(fmt.Sprintf("no releases found for %s/%s", owner, repo))
	}
	return latestFromList(releases), nil
}

// latestFromList picks the newest release from a pre-fetched list using the
// same date-based sort key as the Python version.
func latestFromList(releases []map[string]any) map[string]any {
	re := regexp.MustCompile(`-(\d{6})-`)
	best := releases[0]
	bestKey := "000000"
	for _, r := range releases {
		tag, _ := r["tag_name"].(string)
		if m := re.FindStringSubmatch(tag); m != nil {
			if m[1] > bestKey {
				bestKey = m[1]
				best = r
			}
		}
	}
	return best
}

// FindAsset searches a release's asset list for the first asset whose name
// contains pattern. Returns the asset name, download URL, and whether it was
// found.
func FindAsset(release map[string]any, pattern string) (name, url string, found bool) {
	assets, _ := release["assets"].([]any)
	for _, a := range assets {
		asset, _ := a.(map[string]any)
		assetName, _ := asset["name"].(string)
		if len(assetName) >= len(pattern) {
			// match suffix (same as Python endswith)
			if len(assetName) >= len(pattern) && assetName[len(assetName)-len(pattern):] == pattern {
				dl, _ := asset["browser_download_url"].(string)
				return assetName, dl, true
			}
		}
	}
	return "", "", false
}

// DownloadFile downloads url to destPath, printing progress to stdout.
// The destination directory is created automatically if it does not exist.
func DownloadFile(url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nterrors.FirmwareError("creating destination directory: " + err.Error())
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nterrors.FirmwareError("building download request: " + err.Error())
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nterrors.FirmwareError("download request failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nterrors.FirmwareError(fmt.Sprintf("download returned HTTP %d", resp.StatusCode))
	}

	total := resp.ContentLength // -1 if unknown

	f, err := os.Create(destPath)
	if err != nil {
		return nterrors.FirmwareError("creating destination file: " + err.Error())
	}
	defer f.Close()

	buf := make([]byte, 128*1024)
	var done int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return nterrors.FirmwareError("writing file: " + writeErr.Error())
			}
			done += int64(n)
			if total > 0 {
				pct := done * 100 / total
				fmt.Printf("\r  %d%%  %.1f / %.1f MB",
					pct,
					float64(done)/1024/1024,
					float64(total)/1024/1024,
				)
			} else {
				fmt.Printf("\r  %.1f MB downloaded", float64(done)/1024/1024)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nterrors.FirmwareError("reading download stream: " + readErr.Error())
		}
	}
	fmt.Println()
	return nil
}
