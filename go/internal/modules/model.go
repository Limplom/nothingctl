// Package modules provides Magisk module list, download, and install helpers.
package modules

// ModuleInfo describes a Magisk module from modules.json.
type ModuleInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Source         string `json:"source"`
	InstallType    string `json:"install_type"`
	Repo           string `json:"repo"`
	AssetPattern   string `json:"asset_pattern"`
	RequiresZygisk bool   `json:"requires_zygisk"`
	UsePrerelease  bool   `json:"use_prerelease"`
	Notes          string `json:"notes"`
}
