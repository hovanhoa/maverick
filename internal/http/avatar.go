package http

import (
	"io"
	"net/http"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http"
)

// allowedAvatarContentTypes bounds what an uploaded avatar may claim to be -
// a small, common image set, not an arbitrary file upload.
var allowedAvatarContentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// getAvatar serves an account's profile picture. Deliberately unauthenticated
// (unlike every other route here): an <img> tag can't attach an Authorization
// header, and a profile picture isn't sensitive, so this only needs the
// account id, the same as any other avatar URL.
func (s *Service) getAvatar(c *corehttp.Context) {
	avatar, err := s.deps.DB.GetAccountAvatar(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if avatar == nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Writer.Header().Set("Content-Type", avatar.ContentType)
	c.Writer.Header().Set("Cache-Control", "private, max-age=300")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(avatar.Data)
}

// uploadAvatar sets the caller's own profile picture. Self-service only -
// there is no OWNER/ADMIN-on-behalf-of path, unlike updateAccount, since a
// profile picture is inherently personal.
func (s *Service) uploadAvatar(c *corehttp.Context) {
	principal := auth.GetPrincipal[model.Identity, model.Role](c.Request.Context())
	if principal == nil || principal.ID != c.Param("id") {
		c.JSON(http.StatusForbidden, map[string]string{"error": "you may only set your own avatar"})
		return
	}

	contentType := c.Request.Header.Get("Content-Type")
	if !allowedAvatarContentTypes[contentType] {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "avatar must be image/png, image/jpeg, image/webp, or image/gif"})
		return
	}

	data, err := io.ReadAll(io.LimitReader(c.Request.Body, db.MaxAvatarBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "avatar image is empty"})
		return
	}
	if len(data) > db.MaxAvatarBytes {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "avatar image is too large (max 2 MiB)"})
		return
	}

	if err := s.deps.DB.SetAccountAvatar(c.Request.Context(), principal.ID, contentType, data); err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

// deleteAvatar removes the caller's own profile picture, if any. Self-service
// only, same as uploadAvatar.
func (s *Service) deleteAvatar(c *corehttp.Context) {
	principal := auth.GetPrincipal[model.Identity, model.Role](c.Request.Context())
	if principal == nil || principal.ID != c.Param("id") {
		c.JSON(http.StatusForbidden, map[string]string{"error": "you may only remove your own avatar"})
		return
	}

	if _, err := s.deps.DB.DeleteAccountAvatar(c.Request.Context(), principal.ID); err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}
