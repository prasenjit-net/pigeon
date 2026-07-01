package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/config"
	"github.com/prasenjit-net/pigeon/internal/registry"
	"github.com/prasenjit-net/pigeon/internal/version"
)

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

// Handler holds dependencies for all API request handlers.
type Handler struct {
	config   config.Config
	version  version.Info
	ca       *ca.CA
	registry registry.Registry
}

func NewHandler(cfg config.Config, build version.Info, authority *ca.CA, reg registry.Registry) *Handler {
	return &Handler{config: cfg, version: build, ca: authority, registry: reg}
}

// --- health / meta ---------------------------------------------------------

type healthResponse struct {
	Status  string       `json:"status"`
	Service string       `json:"service"`
	Env     string       `json:"env"`
	Time    time.Time    `json:"time"`
	Version version.Info `json:"version"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: h.config.App.Name,
		Env:     h.config.App.Env,
		Time:    time.Now().UTC(),
		Version: h.version,
	})
}

type metaResponse struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Environment string       `json:"environment"`
	URL         string       `json:"url"`
	Version     version.Info `json:"version"`
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, metaResponse{
		Name:        h.config.App.Name,
		Description: h.config.App.Description,
		Environment: h.config.App.Env,
		URL:         h.config.App.URL,
		Version:     h.version,
	})
}

// --- CA public key ---------------------------------------------------------

func (h *Handler) CAPublicKey(w http.ResponseWriter, r *http.Request) {
	jwk, err := h.ca.PublicKeyJWK()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get CA public key")
		return
	}
	respondJSON(w, http.StatusOK, jwk)
}

// --- registration ----------------------------------------------------------

type registerRequest struct {
	Handle        string         `json:"handle"`
	Name          string         `json:"name"`
	ID            string         `json:"id"` // hex SHA-256 of signing key JWK
	SigningKey     map[string]any `json:"signingKey"`
	EncryptionKey map[string]any `json:"encryptionKey"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if !handleRe.MatchString(req.Handle) {
		respondError(w, http.StatusBadRequest, "handle must be 3–32 characters: lowercase letters, digits, underscores; must start with a letter")
		return
	}
	if req.Name == "" || len(req.Name) > 64 {
		respondError(w, http.StatusBadRequest, "name must be 1–64 characters")
		return
	}
	if req.ID == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.SigningKey == nil || req.EncryptionKey == nil {
		respondError(w, http.StatusBadRequest, "signingKey and encryptionKey are required")
		return
	}

	// Soft uniqueness check before issuing the cert (race-safe: DB unique index is the hard guard).
	if _, err := h.registry.GetByHandle(req.Handle); err == nil {
		respondError(w, http.StatusConflict, "handle already taken")
		return
	}

	signed, err := h.ca.Issue(req.Handle, req.Name, req.ID, req.SigningKey, req.EncryptionKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "certificate issuance failed")
		return
	}

	if err := h.registry.Register(signed); err != nil {
		if isConflict(err) {
			respondError(w, http.StatusConflict, "handle already taken")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to store registration")
		return
	}

	respondJSON(w, http.StatusCreated, signed)
}

// --- user search -----------------------------------------------------------

type searchResult struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}

// SearchUser looks up a single user by handle.
// Returns {id, handle, name} only — no keys. Keys are delivered via the WS
// roster after a connection request is accepted.
func (h *Handler) SearchUser(w http.ResponseWriter, r *http.Request) {
	handle := r.URL.Query().Get("handle")
	if handle == "" {
		respondError(w, http.StatusBadRequest, "handle query parameter is required")
		return
	}
	cert, err := h.registry.GetByHandle(handle)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	respondJSON(w, http.StatusOK, searchResult{
		ID:     cert.Cert.Subject.ID,
		Handle: cert.Cert.Subject.Handle,
		Name:   cert.Cert.Subject.Name,
	})
}

// isConflict returns true for duplicate-key DB errors.
func isConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key value")
}

// --- helpers ---------------------------------------------------------------

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}
