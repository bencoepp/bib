package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

func UpdateBib(version string, option *Option) error {
	log.Info("🔍 Checking for updates...",
		"current", version,
		"repo", fmt.Sprintf("%s/%s", option.Owner, option.Repo),
	)

	ctx, cancel := context.WithTimeout(context.Background(), option.HTTPTimeout)
	defer cancel()

	current, err := ParseVersion(version)
	if err != nil {
		log.Warn("⚠️ Current version is not a valid semver; continuing best-effort.", "version", version, "err", err)
	}

	rel, latest, err := FetchLatestMatchingRelease(ctx, option.Owner, option.Repo, option.AllowPrerelease, option.BinaryName)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}
	if rel == nil || latest == nil {
		log.Info("ℹ️  No matching release found with name pattern 'bib-v[x.y.z]'.")
		return nil
	}

	if current != nil && CompareVersions(latest, current) <= 0 {
		log.Info("✅ Already up to date.",
			"latest", latest.String(),
		)
		return nil
	}

	log.Info("⬆️  Update available!",
		"latest", latest.String(),
		"release", rel.Htmlurl,
	)

	asset, err := PickAssetForPlatform(rel.Assets, option.BinaryName)
	if err != nil {
		return fmt.Errorf("pick asset for platform: %w", err)
	}
	log.Info("📦 Selected asset", "name", asset.Name)

	tmpDir, err := os.MkdirTemp("", "bib-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	assetPath := filepath.Join(tmpDir, asset.Name)
	log.Info("⏬ Downloading asset...", "dest", assetPath)
	if err := DownloadFile(ctx, asset.BrowserDownloadURL, assetPath, option.BinaryName); err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	log.Info("✅ Downloaded.")

	log.Info("🔐 Verifying checksum (if available)...")
	if err := VerifyChecksumIfAvailable(ctx, rel.Assets, asset.Name, assetPath, tmpDir, option.BinaryName); err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}

	log.Info("🧩 Extracting binary from asset (if archive)...")
	newBinPath, err := MaterializeBinaryFromAsset(assetPath, tmpDir, option.BinaryName)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}
	log.Info("✅ Prepared new binary.", "path", newBinPath)

	log.Info("🛠  Installing update...")
	if err := InstallBinary(newBinPath, option.BinaryName); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	log.Info("🎉 Update complete.")
	return nil
}
