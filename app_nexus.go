package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/nexus"
	"github.com/Kuusouu/Cratebug/internal/secret"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const nexusLinkEvent = "nexus:link"

// Bound to the stored API key so a blob written for another purpose cannot
// be decrypted as this value.
var nexusKeyEntropy = []byte("Cratebug/nexus-api-key/v1")

type pendingNexusInstall struct {
	ModID   int
	FileID  int
	Version string
}

// Whether an API key file exists. The key itself is never returned.
type NexusKeyState struct {
	Configured bool `json:"configured"`
}

// Connected-account details safe to show in the WebView. Never includes
// the API key.
type NexusAccountState struct {
	Configured      bool   `json:"configured"`
	Verified        bool   `json:"verified"`
	Name            string `json:"name,omitempty"`
	IsPremium       bool   `json:"isPremium"`
	HourlyRemaining int    `json:"hourlyRemaining"`
	DailyRemaining  int    `json:"dailyRemaining"`
	HourlyResetUnix int64  `json:"hourlyResetUnix"`
	DailyResetUnix  int64  `json:"dailyResetUnix"`
}

// One file attached to a Nexus mod, for the install picker.
type NexusFileSummary struct {
	FileID       int    `json:"fileId"`
	Name         string `json:"name"`
	FileName     string `json:"fileName"`
	Version      string `json:"version"`
	SizeKB       int    `json:"sizeKb"`
	CategoryName string `json:"categoryName"`
	IsPrimary    bool   `json:"isPrimary"`
}

// A resolved Nexus page or pending nxm:// link with no signing secrets.
type NexusLink struct {
	Present      bool               `json:"present"`
	Game         string             `json:"game,omitempty"`
	ModID        int                `json:"modId,omitempty"`
	FileID       int                `json:"fileId,omitempty"`
	ModName      string             `json:"modName,omitempty"`
	Author       string             `json:"author,omitempty"`
	PictureURL   string             `json:"pictureUrl,omitempty"`
	FileName     string             `json:"fileName,omitempty"`
	Version      string             `json:"version,omitempty"`
	SizeKB       int                `json:"sizeKb,omitempty"`
	Premium      bool               `json:"premium"`
	NeedsWebsite bool               `json:"needsWebsite"`
	Files        []NexusFileSummary `json:"files,omitempty"`
}

const (
	nexusFileCategoryMain     = "MAIN"
	nexusFileCategoryOptional = "OPTIONAL"
)

// Anchors nexus.DownloadRequest so Wails emits its TypeScript model.
func (a *App) NexusDownloadRequestType() nexus.DownloadRequest {
	return nexus.DownloadRequest{}
}

// Reports whether a Nexus API key file is present, without decrypting it.
func (a *App) NexusKeyState() NexusKeyState {
	return NexusKeyState{Configured: a.secretStore.Configured()}
}

// Validates key against the Nexus API and stores it only after the account
// is confirmed. An unverified key is rejected and not written.
func (a *App) SetNexusAPIKey(key string) (NexusKeyState, error) {
	if err := nexus.CheckAPIKey(key); err != nil {
		return a.NexusKeyState(), err
	}

	client := a.newNexusClient(key)
	if _, err := client.Validate(a.requestContext()); err != nil {
		return a.NexusKeyState(), err
	}
	if err := a.secretStore.Set(key); err != nil {
		return a.NexusKeyState(), err
	}

	a.nexusMu.Lock()
	a.nexusClient = client
	a.nexusMu.Unlock()
	return NexusKeyState{Configured: true}, nil
}

// Removes the stored API key and drops the cached client.
func (a *App) ClearNexusAPIKey() (NexusKeyState, error) {
	if err := a.secretStore.Clear(); err != nil {
		return a.NexusKeyState(), err
	}
	a.nexusMu.Lock()
	a.nexusClient = nil
	a.nexusMu.Unlock()
	return NexusKeyState{Configured: false}, nil
}

// Loads the connected account without exposing the API key. A file that
// exists but cannot be decrypted is configured and unverified so Settings
// can ask the user to re-enter the key.
func (a *App) NexusAccount() NexusAccountState {
	if !a.secretStore.Configured() {
		return NexusAccountState{}
	}

	client, err := a.requireNexusClient()
	if err != nil {
		return NexusAccountState{Configured: true}
	}
	user, err := client.Validate(a.requestContext())
	if err != nil {
		return NexusAccountState{Configured: true}
	}
	limits := client.RateLimit()
	return NexusAccountState{
		Configured:      true,
		Verified:        true,
		Name:            user.Name,
		IsPremium:       user.IsPremium,
		HourlyRemaining: limits.HourlyRemaining,
		DailyRemaining:  limits.DailyRemaining,
		HourlyResetUnix: unixOrZero(limits.HourlyReset),
		DailyResetUnix:  unixOrZero(limits.DailyReset),
	}
}

// Parses a Nexus Mods site URL and fetches display metadata for it.
func (a *App) ResolveNexusModPage(rawURL string) (NexusLink, error) {
	page, err := nexus.ParseModPageURL(rawURL)
	if err != nil {
		return NexusLink{}, err
	}
	return a.resolveDownloadRequest(nexus.DownloadRequest{
		Game:    page.Game,
		ModID:   page.ModID,
		FileID:  page.FileID,
		Premium: true,
	})
}

// Drains the pending nxm:// buffer once. An empty buffer returns Present
// false rather than an error so a mount-time pull is cheap.
func (a *App) TakePendingNexusLink() (NexusLink, error) {
	a.pendingURLMu.Lock()
	req := a.pendingLink
	a.pendingLink = nil
	a.pendingURLMu.Unlock()
	if req == nil {
		return NexusLink{Present: false}, nil
	}
	return a.resolveDownloadRequest(*req)
}

// Drops signed nxm:// parameters for one mod/file pair.
func (a *App) DiscardNexusLink(modID, fileID int) {
	a.dropLinkSecrets(modID, fileID)
}

// Downloads a Nexus file through the API and stages it for the existing
// install preview. Signed parameters are used when a matching nxm://
// link stored them; otherwise the premium/keyless path is used.
func (a *App) PrepareNexusInstall(modRoot string, modID, fileID int, defaultFolder string) (install.PreviewResult, error) {
	client, err := a.requireNexusClient()
	if err != nil {
		return install.PreviewResult{}, err
	}

	ctx, cancel := context.WithCancel(a.requestContext())
	a.nexusMu.Lock()
	a.nexusDownloadCancel = cancel
	a.nexusMu.Unlock()
	defer func() {
		a.nexusMu.Lock()
		a.nexusDownloadCancel = nil
		a.nexusMu.Unlock()
		cancel()
	}()

	file, err := client.File(ctx, modID, fileID)
	if err != nil {
		return install.PreviewResult{}, err
	}

	secrets, _ := a.peekLinkSecrets(modID, fileID)
	links, err := client.DownloadLinks(ctx, modID, fileID, secrets)
	if err != nil {
		return install.PreviewResult{}, err
	}
	if len(links) == 0 {
		return install.PreviewResult{}, fmt.Errorf("nexus: no download links")
	}
	a.dropLinkSecrets(modID, fileID)

	onProgress := func(p install.Progress) {
		a.emitInstallProgress(p)
	}
	downloadedPath, cleanup, err := nexus.Download(ctx, links[0], file, nil, onProgress)
	if err != nil {
		return install.PreviewResult{}, err
	}
	defer cleanup()

	preview, err := a.stageAndPreview(modRoot, []string{downloadedPath}, defaultFolder)
	if err != nil {
		return preview, err
	}

	version := file.ModVersion
	if version == "" {
		version = file.Version
	}
	a.setPendingNexusSource(pendingNexusInstall{
		ModID:   modID,
		FileID:  fileID,
		Version: version,
	})
	return preview, nil
}

// Stops an in-progress Nexus download. Idle calls are a no-op.
func (a *App) CancelNexusDownload() {
	a.nexusMu.Lock()
	defer a.nexusMu.Unlock()
	if a.nexusDownloadCancel != nil {
		a.nexusDownloadCancel()
		a.nexusDownloadCancel = nil
	}
}

// Stores a cold-start or second-instance nxm:// URL. Signing parameters
// stay in Go. Not exported: Wails would otherwise bind it.
func (a *App) setLaunchURL(rawURL string) {
	req, secrets, err := nexus.ParseDownloadURL(rawURL)
	if err != nil {
		return
	}

	a.pendingURLMu.Lock()
	a.pendingLink = &req
	if a.linkSecrets == nil {
		a.linkSecrets = make(map[string]nexus.DownloadSecrets)
	}
	if secrets.Key != "" || secrets.Expires != "" {
		a.linkSecrets[linkSecretKey(req.ModID, req.FileID)] = secrets
	}
	ready := a.ctxReady
	a.pendingURLMu.Unlock()

	if ready {
		a.emitNexusLink(req)
	}
}

func (a *App) emitNexusLink(req nexus.DownloadRequest) {
	a.pendingURLMu.Lock()
	ready := a.ctxReady
	ctx := a.ctx
	a.pendingURLMu.Unlock()
	if !ready || ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(ctx, nexusLinkEvent, req)
}

func (a *App) resolveDownloadRequest(req nexus.DownloadRequest) (NexusLink, error) {
	link := NexusLink{
		Present: true,
		Game:    req.Game,
		ModID:   req.ModID,
		FileID:  req.FileID,
		Premium: req.Premium,
	}

	client, err := a.requireNexusClient()
	if err != nil {
		if errors.Is(err, nexus.ErrNoAPIKey) {
			return link, nil
		}
		return NexusLink{}, err
	}

	ctx := a.requestContext()
	mod, err := client.Mod(ctx, req.ModID)
	if err != nil {
		return NexusLink{}, err
	}
	link.ModName = mod.Name
	link.Author = mod.Author
	link.PictureURL = mod.PictureURL
	link.Version = mod.Version

	if req.FileID > 0 {
		file, fileErr := client.File(ctx, req.ModID, req.FileID)
		if fileErr != nil {
			return NexusLink{}, fileErr
		}
		link.FileName = file.Name
		link.SizeKB = file.SizeKB
		if file.ModVersion != "" {
			link.Version = file.ModVersion
		} else if file.Version != "" {
			link.Version = file.Version
		}
	} else {
		files, filesErr := client.Files(ctx, req.ModID)
		if filesErr != nil {
			return NexusLink{}, filesErr
		}
		link.Files = summarizeInstallableFiles(files)
	}

	user, userErr := client.Validate(ctx)
	if userErr == nil {
		_, hasSecrets := a.peekLinkSecrets(req.ModID, req.FileID)
		link.NeedsWebsite = !user.IsPremium && !hasSecrets
	}
	return link, nil
}

func (a *App) requireNexusClient() (*nexus.Client, error) {
	key, err := a.secretStore.Get()
	if errors.Is(err, secret.ErrNotConfigured) {
		return nil, nexus.ErrNoAPIKey
	}
	if err != nil {
		return nil, fmt.Errorf("read nexus API key: %w", err)
	}
	if err := nexus.CheckAPIKey(key); err != nil {
		return nil, err
	}

	a.nexusMu.Lock()
	defer a.nexusMu.Unlock()
	if a.nexusClient != nil && a.nexusClient.APIKey == key {
		return a.nexusClient, nil
	}
	client := a.newNexusClient(key)
	a.nexusClient = client
	return client, nil
}

func (a *App) newNexusClient(key string) *nexus.Client {
	client := nexus.NewClient(key, AppVersion)
	if a.nexusBaseURL != "" {
		client.BaseURL = a.nexusBaseURL
	}
	return client
}

func (a *App) requestContext() context.Context {
	a.pendingURLMu.Lock()
	ctx := a.ctx
	a.pendingURLMu.Unlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (a *App) emitInstallProgress(p install.Progress) {
	a.pendingURLMu.Lock()
	ctx := a.ctx
	a.pendingURLMu.Unlock()
	if ctx != nil {
		wailsRuntime.EventsEmit(ctx, "install:progress", p)
	}
}

func (a *App) peekLinkSecrets(modID, fileID int) (nexus.DownloadSecrets, bool) {
	a.pendingURLMu.Lock()
	defer a.pendingURLMu.Unlock()
	secrets, ok := a.linkSecrets[linkSecretKey(modID, fileID)]
	return secrets, ok
}

func (a *App) dropLinkSecrets(modID, fileID int) {
	a.pendingURLMu.Lock()
	defer a.pendingURLMu.Unlock()
	delete(a.linkSecrets, linkSecretKey(modID, fileID))
}

func (a *App) setPendingNexusSource(source pendingNexusInstall) {
	a.nexusMu.Lock()
	copy := source
	a.pendingNexusSource = &copy
	a.nexusMu.Unlock()
}

func (a *App) takePendingNexusSource() *pendingNexusInstall {
	a.nexusMu.Lock()
	defer a.nexusMu.Unlock()
	source := a.pendingNexusSource
	a.pendingNexusSource = nil
	return source
}

func (a *App) clearPendingNexusSource() {
	a.nexusMu.Lock()
	a.pendingNexusSource = nil
	a.nexusMu.Unlock()
}

func linkSecretKey(modID, fileID int) string {
	return strconv.Itoa(modID) + "/" + strconv.Itoa(fileID)
}

func summarizeInstallableFiles(files []nexus.FileInfo) []NexusFileSummary {
	summaries := make([]NexusFileSummary, 0, len(files))
	for _, file := range files {
		if !isInstallableNexusFile(file) {
			continue
		}
		summaries = append(summaries, NexusFileSummary{
			FileID:       file.FileID,
			Name:         file.Name,
			FileName:     file.FileName,
			Version:      firstNonEmpty(file.ModVersion, file.Version),
			SizeKB:       file.SizeKB,
			CategoryName: file.CategoryName,
			IsPrimary:    file.IsPrimary,
		})
	}
	return summaries
}

func isInstallableNexusFile(file nexus.FileInfo) bool {
	return strings.EqualFold(file.CategoryName, nexusFileCategoryMain) ||
		strings.EqualFold(file.CategoryName, nexusFileCategoryOptional)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
