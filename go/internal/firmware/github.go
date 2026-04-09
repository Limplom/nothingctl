// Package firmware provides GitHub API helpers, firmware download, extraction,
// and resolution for Nothing Phone devices.
package firmware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const (
	userAgent = "nothing-firmware-manager/2.0"
)

var dateTagRe = regexp.MustCompile(`-(\d{6})-`)

// GhGetCtx performs an HTTP GET against the nothing_archive GitHub API and
// returns the raw response body. It handles rate limiting by returning an error
// with the rate-limit reset time when a 403/429 is received. The request is
// bound to ctx so callers can cancel or time-out the call.
func GhGetCtx(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

// GhGet is a convenience shim around GhGetCtx that uses context.Background().
func GhGet(url string) ([]byte, error) {
	return GhGetCtx(context.Background(), url)
}

// FetchReleasesCtx fetches up to 50 GitHub releases for owner/repo and returns
// them as a slice of raw JSON objects. The request is bound to ctx.
func FetchReleasesCtx(ctx context.Context, owner, repo string) ([]map[string]any, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=50", owner, repo)
	body, err := GhGetCtx(ctx, url)
	if err != nil {
		return nil, err
	}
	var releases []map[string]any
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, nterrors.FirmwareError("parsing releases JSON: " + err.Error())
	}
	return releases, nil
}

// FetchReleases is a convenience shim around FetchReleasesCtx that uses
// context.Background().
func FetchReleases(owner, repo string) ([]map[string]any, error) {
	return FetchReleasesCtx(context.Background(), owner, repo)
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
	best := releases[0]
	bestKey := "000000"
	for _, r := range releases {
		tag, _ := r["tag_name"].(string)
		if m := dateTagRe.FindStringSubmatch(tag); m != nil {
			if m[1] > bestKey {
				bestKey = m[1]
				best = r
			}
		}
	}
	return best
}

// FetchLatestReleaseCtx returns the latest non-prerelease GitHub release map
// and its tag string for the given device codename. It queries nothing_archive,
// filters releases whose tag starts with "<codename>_", and picks the newest
// one by the 6-digit date key embedded in the tag. The request is bound to ctx.
func FetchLatestReleaseCtx(ctx context.Context, codename string) (release map[string]any, latestTag string, err error) {
	releases, err := FetchReleasesCtx(ctx, nothingArchiveOwner, nothingArchiveRepo)
	if err != nil {
		return nil, "", fmt.Errorf("cannot reach GitHub API: %w", err)
	}

	prefix := strings.ToLower(codename) + "_"
	var matched []map[string]any
	for _, r := range releases {
		tag, _ := r["tag_name"].(string)
		if strings.HasPrefix(strings.ToLower(tag), prefix) {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return nil, "", fmt.Errorf("no releases found for codename '%s' in nothing_archive", codename)
	}

	latest := latestFromList(matched)
	tag, _ := latest["tag_name"].(string)
	return latest, tag, nil
}

// FetchLatestRelease is a convenience shim around FetchLatestReleaseCtx that
// uses context.Background().
func FetchLatestRelease(codename string) (release map[string]any, latestTag string, err error) {
	return FetchLatestReleaseCtx(context.Background(), codename)
}

// FindAsset searches a release's asset list for the first asset whose name
// contains pattern. Returns the asset name, download URL, and whether it was
// found.
func FindAsset(release map[string]any, pattern string) (name, url string, found bool) {
	assets, ok := release["assets"].([]any)
	if !ok {
		return "", "", false
	}
	for _, a := range assets {
		asset, ok := a.(map[string]any)
		if !ok {
			continue
		}
		assetName, _ := asset["name"].(string)
		if strings.HasSuffix(assetName, pattern) {
			dl, _ := asset["browser_download_url"].(string)
			return assetName, dl, true
		}
	}
	return "", "", false
}

// DownloadFileCtx downloads url to destPath. The request is bound to ctx so
// callers can cancel or time-out the download. If progressFn is non-nil it is
// called after each chunk with the number of bytes downloaded so far and the
// total content length (-1 when unknown). The destination directory is created
// automatically if it does not exist.
func DownloadFileCtx(ctx context.Context, url, destPath string, progressFn func(downloaded, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nterrors.FirmwareError("creating destination directory: " + err.Error())
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
			if progressFn != nil {
				progressFn(done, total)
			} else if total > 0 {
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
	if progressFn == nil {
		fmt.Println()
	}
	return nil
}

// DownloadFile downloads url to destPath, printing progress to stdout.
// It is a convenience shim around DownloadFileCtx that uses context.Background()
// and the built-in stdout progress printer.
// The destination directory is created automatically if it does not exist.
func DownloadFile(url, destPath string) error {
	return DownloadFileCtx(context.Background(), url, destPath, nil)
}
