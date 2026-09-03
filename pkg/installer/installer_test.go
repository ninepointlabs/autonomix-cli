package installer

import (
	"testing"
	"github.com/ninepointlabs/autonomix-cli/pkg/github"
	"github.com/ninepointlabs/autonomix-cli/pkg/system"
	"github.com/ninepointlabs/autonomix-cli/pkg/packages"
)

func TestGetCompatibleAssets_Universal(t *testing.T) {
	sysType := system.GetSystemPreferredType()
	if sysType == packages.Unknown {
		t.Skip("Skipping test as no package manager detected")
	}

	var assetName string
	switch sysType {
	case packages.Deb:
		assetName = "app_1.0.0_all.deb"
	case packages.Rpm:
		assetName = "app-1.0.0-noarch.rpm"
	case packages.Pacman:
		assetName = "app-1.0.0-any.pkg.tar.zst"
	default:
		t.Skipf("Skipping test for system type %s", sysType)
	}

	release := &github.Release{
		TagName: "v1.0.0",
		Assets: []github.Asset{
			{Name: assetName, BrowserDownloadURL: "http://example.com/" + assetName},
			{Name: "other_arch_asset.zip", BrowserDownloadURL: "http://example.com/bad"},
		},
	}

	assets, err := GetCompatibleAssets(release)
	if err != nil {
		t.Fatalf("GetCompatibleAssets returned error: %v", err)
	}

	if len(assets) == 0 {
		t.Errorf("Expected to find compatible asset %s, but found none", assetName)
	} else {
		found := false
		for _, a := range assets {
			if a.Name == assetName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find asset %s, but got %v", assetName, assets)
		}
	}
}
