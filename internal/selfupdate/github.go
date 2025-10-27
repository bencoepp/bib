package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type GHAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GHRelease struct {
	Name       string    `json:"name"`
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []GHAsset `json:"assets"`
	Htmlurl    string    `json:"html_url"`
	Published  string    `json:"published_at"`
}

// SemVer is a minimal semantic version representation: major.minor.patch[-pre]
// Build metadata (+...) is ignored for comparisons.
type SemVer struct {
	Major int
	Minor int
	Patch int
	Pre   string // empty means release
}

func (v *SemVer) String() string {
	if v == nil {
		return ""
	}
	if v.Pre == "" {
		return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("v%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Pre)
}

// ParseVersion accepts v-prefixed or plain versions, like "v1.2.3", "1.2.3", "1.2.3-rc.1".
func ParseVersion(s string) (*SemVer, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "v")
	// Drop build metadata if present
	if i := strings.Index(t, "+"); i >= 0 {
		t = t[:i]
	}
	pre := ""
	if i := strings.Index(t, "-"); i >= 0 {
		pre = t[i+1:]
		t = t[:i]
	}
	parts := strings.Split(t, ".")
	if len(parts) < 3 {
		return nil, fmt.Errorf("not semver core: %q", s)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	pat, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, err
	}
	return &SemVer{Major: maj, Minor: min, Patch: pat, Pre: pre}, nil
}

// CompareVersions compares a and b. Returns 1 if a>b, 0 if equal, -1 if a<b.
// Pre-release is considered lower than the release with same core.
func CompareVersions(a, b *SemVer) int {
	if a == nil || b == nil {
		return 0
	}
	if a.Major != b.Major {
		if a.Major > b.Major {
			return 1
		}
		return -1
	}
	if a.Minor != b.Minor {
		if a.Minor > b.Minor {
			return 1
		}
		return -1
	}
	if a.Patch != b.Patch {
		if a.Patch > b.Patch {
			return 1
		}
		return -1
	}
	// same core, handle pre
	if a.Pre == b.Pre {
		return 0
	}
	if a.Pre == "" && b.Pre != "" {
		return 1 // release > pre
	}
	if a.Pre != "" && b.Pre == "" {
		return -1 // pre < release
	}
	// both pre: simple lexicographic comparison
	if a.Pre > b.Pre {
		return 1
	}
	if a.Pre < b.Pre {
		return -1
	}
	return 0
}

// bib-v1.2.3 or bib-v1.2.3-rc.1
var relNameRx = regexp.MustCompile(`\bbib-v(\d+\.\d+\.\d+(?:-[A-Za-z0-9\.-]+)?)\b`)

// FetchLatestMatchingRelease gets the latest non-draft release where the Name contains "bib-v[VERSION]".
// If includePre is false, prereleases are skipped.
func FetchLatestMatchingRelease(ctx context.Context, owner, repo string, includePre bool, binaryName string) (*GHRelease, *SemVer, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	rels, err := getJSON[[]GHRelease](ctx, url, binaryName)
	if err != nil {
		return nil, nil, err
	}

	var bestRel *GHRelease
	var bestVer *SemVer

	for i := range rels {
		r := &rels[i]
		if r.Draft {
			continue
		}
		if r.Prerelease && !includePre {
			continue
		}
		m := relNameRx.FindStringSubmatch(r.Name)
		if len(m) != 2 {
			continue
		}
		v, err := ParseVersion(m[1])
		if err != nil {
			continue
		}
		if bestVer == nil || CompareVersions(v, bestVer) > 0 {
			bestVer = v
			bestRel = r
		}
	}

	return bestRel, bestVer, nil
}

// PickAssetForPlatform picks an asset whose name best matches GOOS/GOARCH and common extensions.
func PickAssetForPlatform(assets []GHAsset, binaryName string) (*GHAsset, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	archSyn := []string{goarch}
	switch goarch {
	case "amd64":
		archSyn = append(archSyn, "x86_64", "x64")
	case "arm64":
		archSyn = append(archSyn, "aarch64")
	}

	osSyn := []string{goos}
	switch goos {
	case "darwin":
		osSyn = append(osSyn, "macos", "mac", "osx")
	}

	isWindows := goos == "windows"

	bestScore := -1
	var best *GHAsset
	for i := range assets {
		a := &assets[i]
		nameLower := strings.ToLower(a.Name)

		// Exclude checksum/signature assets here.
		if strings.Contains(nameLower, "sha256") || strings.Contains(nameLower, "checksums") || strings.HasSuffix(nameLower, ".sig") {
			continue
		}

		extOK := false
		if isWindows {
			extOK = strings.HasSuffix(nameLower, ".zip") || strings.HasSuffix(nameLower, ".exe")
		} else {
			extOK = strings.HasSuffix(nameLower, ".tar.gz") || strings.HasSuffix(nameLower, ".tgz") || strings.HasSuffix(nameLower, binaryName)
		}
		if !extOK {
			continue
		}

		osMatch := containsAny(nameLower, osSyn)
		archMatch := containsAny(nameLower, archSyn)
		nameMatch := strings.Contains(nameLower, strings.ToLower(binaryName))

		score := 0
		if osMatch {
			score += 2
		}
		if archMatch {
			score += 2
		}
		if nameMatch {
			score++
		}

		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no release asset found for %s/%s", goos, goarch)
	}
	return best, nil
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// DownloadFile downloads a URL to dest.
func DownloadFile(ctx context.Context, url, dest string, binaryName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	addGitHubHeaders(req, binaryName)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download failed: %s: %s", resp.Status, string(body))
	}

	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// VerifyChecksumIfAvailable tries to find a checksum asset and verify the downloaded asset.
func VerifyChecksumIfAvailable(ctx context.Context, assets []GHAsset, targetAssetName, localPath, tmpDir string, binaryName string) error {
	var checksumAsset *GHAsset
	for i := range assets {
		n := strings.ToLower(assets[i].Name)
		if strings.Contains(n, "checksums") || strings.Contains(n, "sha256sum") || strings.HasSuffix(n, ".sha256") || strings.HasSuffix(n, ".sha256sum") {
			checksumAsset = &assets[i]
			break
		}
	}
	if checksumAsset == nil {
		// Optional: return nil with no verification
		return nil
	}

	csPath := filepath.Join(tmpDir, checksumAsset.Name)
	if err := DownloadFile(ctx, checksumAsset.BrowserDownloadURL, csPath, binaryName); err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}

	expected, err := findExpectedSHA256(csPath, targetAssetName)
	if err != nil {
		return err
	}
	if expected == "" {
		// No entry found; proceed without verification
		return nil
	}

	actual, err := fileSHA256(localPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum mismatch for %s: expected %s got %s", targetAssetName, expected, actual)
	}
	return nil
}

func findExpectedSHA256(checksumFilePath, assetName string) (string, error) {
	f, err := os.Open(checksumFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	assetBase := filepath.Base(assetName)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Formats:
		// <hash>  <filename>
		// SHA256(<filename>)= <hash>
		// <hash>  ./path/<filename>
		l := line
		if strings.HasPrefix(l, "SHA256(") {
			parts := strings.SplitN(l, ")=", 2)
			if len(parts) == 2 {
				name := strings.TrimPrefix(parts[0], "SHA256(")
				name = filepath.Base(strings.TrimSpace(name))
				hash := strings.TrimSpace(parts[1])
				if name == assetBase {
					return strings.ToLower(hash), nil
				}
			}
			continue
		}
		fields := strings.Fields(l)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		fn := fields[len(fields)-1]
		fn = filepath.Base(strings.TrimPrefix(fn, "./"))
		if fn == assetBase {
			return strings.ToLower(hash), nil
		}
	}
	return "", sc.Err()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MaterializeBinaryFromAsset returns a path to the single binary extracted from the given asset.
// Supports raw binaries, .zip, and .tar.gz/.tgz archives.
func MaterializeBinaryFromAsset(assetPath, tmpDir, binaryName string) (string, error) {
	name := strings.ToLower(filepath.Base(assetPath))
	switch {
	case strings.HasSuffix(name, ".zip"):
		return extractFromZip(assetPath, tmpDir, binaryName)
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		return extractFromTarGz(assetPath, tmpDir, binaryName)
	default:
		// Assume it's the binary itself
		out := filepath.Join(tmpDir, binaryName)
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		if err := copyFile(assetPath, out); err != nil {
			return "", err
		}
		if err := os.Chmod(out, 0o755); err != nil {
			return "", err
		}
		return out, nil
	}
}

func extractFromZip(zipPath, tmpDir, binaryName string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	want := binaryName
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		out := filepath.Join(tmpDir, want)
		dst, err := os.Create(out)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dst, rc); err != nil {
			dst.Close()
			return "", err
		}
		dst.Close()
		if err := os.Chmod(out, 0o755); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("binary %q not found in zip archive", want)
}

func extractFromTarGz(tarPath, tmpDir, binaryName string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	want := binaryName
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if h.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(h.Name)
		if base != want {
			continue
		}
		out := filepath.Join(tmpDir, want)
		dst, err := os.Create(out)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dst, tr); err != nil {
			dst.Close()
			return "", err
		}
		dst.Close()
		if err := os.Chmod(out, 0o755); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("binary %q not found in tar.gz archive", want)
}

// InstallBinary replaces the currently running binary with newBinPath.
// - Unix: atomic rename into place
// - Windows: stage alongside as .new and instruct user to finalize on restart
func InstallBinary(newBinPath, binaryName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	target := exe

	if runtime.GOOS == "windows" {
		staged := target + ".new"
		if err := copyFile(newBinPath, staged); err != nil {
			return err
		}
		// Caller can log user instructions.
		return nil
	}

	// Unix-like: atomic replace
	// Ensure new binary is executable
	if err := os.Chmod(newBinPath, 0o755); err != nil {
		return err
	}

	// Copy into same directory first, then rename
	staged := filepath.Join(dir, "."+binaryName+".tmp")
	if err := copyFile(newBinPath, staged); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return err
	}

	if err := os.Rename(staged, target); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func getJSON[T any](ctx context.Context, url string, binaryName string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	addGitHubHeaders(req, binaryName)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return zero, fmt.Errorf("GitHub API error: %s: %s", resp.Status, string(body))
	}
	var out T
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return zero, err
	}
	return out, nil
}

// addGitHubHeaders sets headers for the GitHub API and optional token usage.
// Set GITHUB_TOKEN in the environment to increase rate limits or access private repos.
func addGitHubHeaders(req *http.Request, binaryName string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", binaryName+"-updater")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}
