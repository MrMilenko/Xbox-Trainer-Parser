package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"trainerui/backend/parser"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	APIURL = "https://mobcat.zip/XboxIDs"
	CDNURL = "https://raw.githubusercontent.com/MobCat/MobCats-original-xbox-game-list/main/icon"
)

type App struct {
	ctx *context.Context
}

func NewApp() *App { return &App{} }

// Startup stores the context for later runtime calls.
func (a *App) Startup(ctx context.Context) { a.ctx = &ctx }

// BrowseDir opens a native directory picker and returns the selected path.
func (a *App) BrowseDir(start string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("no context")
	}
	opts := runtime.OpenDialogOptions{
		Title:                "Select a folder with trainers (.etm / .xbtf)",
		DefaultDirectory:     start,
		CanCreateDirectories: false,
	}
	return runtime.OpenDirectoryDialog(*a.ctx, opts)
}

// ParseFile parses a single trainer file.
func (a *App) ParseFile(path string) (*parser.Trainer, error) {
	return parser.ParsePath(path)
}

// ParseDir parses all trainers in a directory (non-recursive).
func (a *App) ParseDir(dir string) ([]*parser.Trainer, error) {
	// expand ~ for convenience
	if len(dir) > 0 && dir[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[1:])
		}
	}
	return parser.ParseDir(dir)
}

// ---- MobCat enrichment (optional) ----

type titleLookup struct {
	XMID     string `json:"XMID"`
	FullName string `json:"Full_Name"`
}

// ResolveTitleName tries MobCat API: https://mobcat.zip/XboxIDs/api.php?id=<TITLEID>
// Returns the Full_Name or empty string when not found.
func (a *App) ResolveTitleName(titleID string) (string, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	url := fmt.Sprintf("%s/api.php?id=%s", APIURL, titleID)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mobcat status %d", resp.StatusCode)
	}
	var res []titleLookup
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res) == 0 {
		return "", nil
	}
	return res[0].FullName, nil
}
