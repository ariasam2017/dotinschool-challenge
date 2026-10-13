package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
)

// context keys
const ctxAdminUser = "adminUser"

// bearerToken pulls the token out of an "Authorization: Bearer <token>" header.
func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

// fail renders a typed error. Used where the error is always an *apperr.Error
// (validation, auth, rate limit) and there is nothing to log.
func fail(c *gin.Context, err error) {
	if ae := apperr.As(err); ae != nil {
		c.JSON(ae.Status, gin.H{"code": ae.Code, "message": ae.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    "INTERNAL_ERROR",
		"message": "خطای داخلی سرور رخ داد.",
	})
}

// failWith renders an error, logging it first when it is not an *apperr.Error so
// unexpected failures are visible server-side but never leak to the client.
func (s *server) failWith(c *gin.Context, err error) {
	if ae := apperr.As(err); ae != nil {
		c.JSON(ae.Status, gin.H{"code": ae.Code, "message": ae.Message})
		return
	}
	s.log.WithError(err).WithField("path", c.Request.URL.Path).Error("unhandled error")
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    "INTERNAL_ERROR",
		"message": "خطای داخلی سرور رخ داد.",
	})
}

// bindJSON decodes the request body, returning a friendly 400 on malformed JSON
// or a 413 when the body exceeds the configured size limit.
func bindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return apperr.New(http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "حجم درخواست بیش از حد مجاز است.")
		}
		return apperr.New(400, "INVALID_JSON", "بدنه درخواست نامعتبر است.")
	}
	return nil
}

// nowMs is epoch milliseconds.
func nowMs() int64 { return time.Now().UnixMilli() }
