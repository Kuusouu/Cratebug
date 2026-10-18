package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/nexus"
	"github.com/Kuusouu/Cratebug/internal/secret"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
)

const nexusLinkEvent = "nexus:link"

// Bound to the stored API key so a blob written for another purpose cannot
// be decrypted as this value.
var nexusKeyEntropy = []byte("Cratebug/nexus-api-key/v1")

// Nexus account, mod page, nxm:// link, and download bindings. Downloads
// hand off to InstallService for staging and the install preview.
type NexusService struct {
	*core
	installs    *InstallService
	secretStore secret.Store

	pendingURLMu sync.Mutex
	pendingLink  *nexus.DownloadRequest
	linkSecrets  map[string]nexus.DownloadSecrets
	ctxReady     bool

	nexusMu             sync.Mutex
	nexusClient         *nexus.Client
	nexusDownloadCancel context.CancelFunc
	nexusBaseURL        string

	protocol      *urlscheme.Registrar
	allowProtocol bool
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

func newNexusService(c *core, installs *InstallService, secretStore secret.Store) *NexusService {
	return &NexusService{
		core:        c,
		installs:    installs,
		secretStore: secretStore,
		linkSecrets: make(map[string]nexus.DownloadSecrets),
	}
}

// Marks the frontend ready, emits a link buffered before startup, and
// claims nxm:// when nothing else owns it. The runtime context must already
// be set, so a link that setLaunchURL sees as ready can always be emitted.
func (s *NexusService) startup() {
	s.pendingURLMu.Lock()
	s.ctxReady = true
	pending := s.pendingLink
	s.pendingURLMu.Unlock()
	if pending != nil {
		s.emitNexusLink(*pending)
	}
	s.ensureNexusProtocol()
}

// Anchors nexus.DownloadRequest so Wails emits its TypeScript model.
func (s *NexusService) NexusDownloadRequestType() nexus.DownloadRequest {
	return nexus.DownloadRequest{}
}

// Reports whether a Nexus API key file is present, without decrypting it.
func (s *NexusService) NexusKeyState() NexusKeyState {
	return NexusKeyState{Configured: s.secretStore.Configured()}
}

// Validates key against the Nexus API and stores it only after the account
// is confirmed. An unverified key is rejected and not written.
func (s *NexusService) SetNexusAPIKey(key string) (NexusKeyState, error) {
	if err := nexus.CheckAPIKey(key); err != nil {
		return s.NexusKeyState(), err
	}

	client := s.newNexusClient(key)
	if _, err := client.Validate(s.runtime.context()); err != nil {
		return s.NexusKeyState(), err
	}
	if err := s.secretStore.Set(key); err != nil {
		return s.NexusKeyState(), err
	}

	s.nexusMu.Lock()
	s.nexusClient = client
	s.nexusMu.Unlock()
	return NexusKeyState{Configured: true}, nil
}

// Removes the stored API key and drops the cached client.
func (s *NexusService) ClearNexusAPIKey() (NexusKeyState, error) {
	if err := s.secretStore.Clear(); err != nil {
		return s.NexusKeyState(), err
	}
	s.nexusMu.Lock()
	s.nexusClient = nil
	s.nexusMu.Unlock()
	return NexusKeyState{Configured: false}, nil
}

// Loads the connected account without exposing the API key. A file that
// exists but cannot be decrypted is configured and unverified so Settings
// can ask the user to re-enter the key.
func (s *NexusService) NexusAccount() NexusAccountState {
	if !s.secretStore.Configured() {
		return NexusAccountState{}
	}

	client, err := s.requireNexusClient()
	if err != nil {
		return NexusAccountState{Configured: true}
	}
	user, err := client.Validate(s.runtime.context())
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
func (s *NexusService) ResolveNexusModPage(rawURL string) (NexusLink, error) {
	page, err := nexus.ParseModPageURL(rawURL)
	if err != nil {
		return NexusLink{}, err
	}
	return s.resolveDownloadRequest(nexus.DownloadRequest{
		Game:    page.Game,
		ModID:   page.ModID,
		FileID:  page.FileID,
		Premium: true,
	})
}

// Drains the pending nxm:// buffer once. An empty buffer returns Present
// false rather than an error so a mount-time pull is cheap.
func (s *NexusService) TakePendingNexusLink() (NexusLink, error) {
	s.pendingURLMu.Lock()
	req := s.pendingLink
	s.pendingLink = nil
	s.pendingURLMu.Unlock()
	if req == nil {
		return NexusLink{Present: false}, nil
	}
	return s.resolveDownloadRequest(*req)
}

// Drops signed nxm:// parameters for one mod/file pair.
func (s *NexusService) DiscardNexusLink(modID, fileID int) {
	s.dropLinkSecrets(modID, fileID)
}

// Downloads a Nexus file through the API and stages it for the existing
// install preview. Signed parameters are used when a matching nxm://
// link stored them; otherwise the premium/keyless path is used.
func (s *NexusService) PrepareNexusInstall(modRoot string, modID, fileID int, defaultFolder string) (install.PreviewResult, error) {
	client, err := s.requireNexusClient()
	if err != nil {
		return install.PreviewResult{}, err
	}

	ctx, cancel := context.WithCancel(s.runtime.context())
	s.nexusMu.Lock()
	s.nexusDownloadCancel = cancel
	s.nexusMu.Unlock()
	defer func() {
		s.nexusMu.Lock()
		s.nexusDownloadCancel = nil
		s.nexusMu.Unlock()
		cancel()
	}()

	mod, err := client.Mod(ctx, modID)
	if err != nil {
		return install.PreviewResult{}, err
	}
	if err := client.CheckAdultContent(ctx, mod); err != nil {
		s.dropLinkSecrets(modID, fileID)
		return install.PreviewResult{}, err
	}

	file, err := client.File(ctx, modID, fileID)
	if err != nil {
		return install.PreviewResult{}, err
	}

	secrets, _ := s.peekLinkSecrets(modID, fileID)
	links, err := client.DownloadLinks(ctx, modID, fileID, secrets)
	if err != nil {
		return install.PreviewResult{}, err
	}
	if len(links) == 0 {
		return install.PreviewResult{}, fmt.Errorf("nexus: no download links")
	}
	s.dropLinkSecrets(modID, fileID)

	onProgress := func(p install.Progress) {
		s.runtime.emit("install:progress", p)
	}
	downloadedPath, cleanup, err := nexus.Download(ctx, links[0], file, nil, onProgress)
	if err != nil {
		return install.PreviewResult{}, err
	}
	defer cleanup()

	version := file.ModVersion
	if version == "" {
		version = file.Version
	}
	return s.installs.stage(modRoot, []string{downloadedPath}, defaultFolder, &nexusSource{
		ModID:   modID,
		FileID:  fileID,
		Version: version,
	})
}

// Stops an in-progress Nexus download. Idle calls are a no-op.
func (s *NexusService) CancelNexusDownload() {
	s.nexusMu.Lock()
	defer s.nexusMu.Unlock()
	if s.nexusDownloadCancel != nil {
		s.nexusDownloadCancel()
		s.nexusDownloadCancel = nil
	}
}

// Stores a cold-start or second-instance nxm:// URL. Signing parameters
// stay in Go. Not exported: Wails would otherwise bind it.
func (s *NexusService) setLaunchURL(rawURL string) {
	req, secrets, err := nexus.ParseDownloadURL(rawURL)
	if err != nil {
		return
	}

	s.pendingURLMu.Lock()
	s.pendingLink = &req
	if s.linkSecrets == nil {
		s.linkSecrets = make(map[string]nexus.DownloadSecrets)
	}
	if secrets.Key != "" || secrets.Expires != "" {
		s.linkSecrets[linkSecretKey(req.ModID, req.FileID)] = secrets
	}
	ready := s.ctxReady
	s.pendingURLMu.Unlock()

	if ready {
		s.emitNexusLink(req)
	}
}

func (s *NexusService) emitNexusLink(req nexus.DownloadRequest) {
	s.pendingURLMu.Lock()
	ready := s.ctxReady
	s.pendingURLMu.Unlock()
	if !ready {
		return
	}
	s.runtime.emit(nexusLinkEvent, req)
}

func (s *NexusService) resolveDownloadRequest(req nexus.DownloadRequest) (NexusLink, error) {
	link := NexusLink{
		Present: true,
		Game:    req.Game,
		ModID:   req.ModID,
		FileID:  req.FileID,
		Premium: req.Premium,
	}

	client, err := s.requireNexusClient()
	if err != nil {
		if errors.Is(err, nexus.ErrNoAPIKey) {
			return link, nil
		}
		return NexusLink{}, err
	}

	ctx := s.runtime.context()
	mod, err := client.Mod(ctx, req.ModID)
	if err != nil {
		return NexusLink{}, err
	}
	if err := client.CheckAdultContent(ctx, mod); err != nil {
		s.dropLinkSecrets(req.ModID, req.FileID)
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
		_, hasSecrets := s.peekLinkSecrets(req.ModID, req.FileID)
		link.NeedsWebsite = !user.IsPremium && !hasSecrets
	}
	return link, nil
}

func (s *NexusService) requireNexusClient() (*nexus.Client, error) {
	key, err := s.secretStore.Get()
	if errors.Is(err, secret.ErrNotConfigured) {
		return nil, nexus.ErrNoAPIKey
	}
	if err != nil {
		return nil, fmt.Errorf("read nexus API key: %w", err)
	}
	if err := nexus.CheckAPIKey(key); err != nil {
		return nil, err
	}

	s.nexusMu.Lock()
	defer s.nexusMu.Unlock()
	if s.nexusClient != nil && s.nexusClient.APIKey == key {
		return s.nexusClient, nil
	}
	client := s.newNexusClient(key)
	s.nexusClient = client
	return client, nil
}

func (s *NexusService) newNexusClient(key string) *nexus.Client {
	client := nexus.NewClient(key, AppVersion)
	if s.nexusBaseURL != "" {
		client.BaseURL = s.nexusBaseURL
	}
	return client
}

func (s *NexusService) peekLinkSecrets(modID, fileID int) (nexus.DownloadSecrets, bool) {
	s.pendingURLMu.Lock()
	defer s.pendingURLMu.Unlock()
	secrets, ok := s.linkSecrets[linkSecretKey(modID, fileID)]
	return secrets, ok
}

func (s *NexusService) dropLinkSecrets(modID, fileID int) {
	s.pendingURLMu.Lock()
	defer s.pendingURLMu.Unlock()
	delete(s.linkSecrets, linkSecretKey(modID, fileID))
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
