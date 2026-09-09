package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/database"
)

// GET /api/admin/backup — streams a consistent copy of the SQLite file made
// with VACUUM INTO (SQLite's safe, non-locking backup mechanism).
func (s *server) handleBackup(c *gin.Context) {
	tmpPath := filepath.Join(s.cfg.DataDir, fmt.Sprintf(".backup-%d-%s.db", nowMs(), uuid.NewString()))
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// VACUUM INTO takes a string literal, not a bound parameter — escape quotes.
	escaped := strings.ReplaceAll(tmpPath, "'", "''")
	if err := s.svc.DB.Exec("VACUUM INTO '" + escaped + "'").Error; err != nil {
		s.failWith(c, err)
		return
	}
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		s.failWith(c, err)
		return
	}

	filename := "dotinschool-backup-" + time.Now().Format("2006-01-02") + ".db"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/octet-stream", content)
}

// POST /api/admin/backup/restore — replaces the live database with an uploaded
// backup, then restarts the process so it reopens the new file cleanly.
//
// Safe to run against a brand-new / empty server: the uploaded file is opened
// and AutoMigrated in a throwaway connection first (database.Validate), so an
// incompatible or corrupt backup is rejected here — the live database is left
// untouched — instead of crash-looping the panel on the next boot. All journal
// sidecars are cleared before the swap so no stale rollback journal from the
// previous database is replayed against the new one.
func (s *server) handleRestore(c *gin.Context) {
	var body struct {
		Data string `json:"data"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	if body.Data == "" {
		fail(c, apperr.InvalidInput("فایل بکاپ ارسال نشده است."))
		return
	}

	buf, err := base64.StdEncoding.DecodeString(body.Data)
	if err != nil || len(buf) < 16 || !strings.HasPrefix(string(buf[:16]), "SQLite format 3") {
		fail(c, apperr.New(400, "INVALID_FILE", "فایل انتخاب‌شده یک فایل دیتابیس SQLite معتبر نیست."))
		return
	}

	dbPath := s.cfg.DBPath()
	stagePath := dbPath + ".incoming"
	database.CleanSidecars(stagePath) // any leftovers from a previous attempt
	if err := os.WriteFile(stagePath, buf, 0o600); err != nil {
		s.failWith(c, err)
		return
	}

	// Prove the upload can actually become this build's database before we throw
	// the live one away. Validate also AutoMigrates the staged file in place, so
	// a backup from a slightly older version is upgraded here rather than on the
	// crash-prone next boot.
	if verr := database.Validate(stagePath, s.log); verr != nil {
		_ = os.Remove(stagePath)
		database.CleanSidecars(stagePath)
		s.log.WithError(verr).Warn("restore: uploaded database failed validation — restore refused")
		fail(c, apperr.New(400, "RESTORE_INCOMPATIBLE",
			"این فایل بکاپ سالم نیست یا با نسخهٔ فعلی سامانه سازگار نیست — بازیابی انجام نشد و داده‌های فعلی دست‌نخورده ماند. بهتر است هر دو سرور روی یک نسخه باشند."))
		return
	}

	safetyPath := filepath.Join(s.cfg.DataDir, fmt.Sprintf("pre-restore-%d.db", nowMs()))
	if cur, rerr := os.ReadFile(dbPath); rerr == nil {
		_ = os.WriteFile(safetyPath, cur, 0o600)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "restarting": true})

	// Swap the file in and exit; systemd / Docker brings the process back up
	// with the restored database. Doing this in-process (reopening the same
	// path over a live handle) is not safe.
	time.AfterFunc(300*time.Millisecond, func() {
		if sqlDB, e := s.svc.DB.DB(); e == nil {
			_ = sqlDB.Close()
		}
		database.CleanSidecars(dbPath)    // stale -journal/-wal/-shm of the OLD db
		database.CleanSidecars(stagePath) // and any left by validation
		if e := os.Rename(stagePath, dbPath); e != nil {
			// Windows / a bind mount can refuse a rename over a path another
			// handle just touched — fall back to a plain copy so the restore
			// still lands.
			if data, re := os.ReadFile(stagePath); re == nil {
				if we := os.WriteFile(dbPath, data, 0o600); we == nil {
					_ = os.Remove(stagePath)
					e = nil
				}
			}
			if e != nil {
				s.log.WithError(e).Errorf("restore: could not swap in the uploaded database — the pre-restore copy is at %s", safetyPath)
			}
		}
		os.Exit(0)
	})
}

// POST /api/admin/update — Docker-aware self-update.
func (s *server) handleUpdate(c *gin.Context) {
	if s.cfg.InDocker {
		c.JSON(http.StatusOK, gin.H{
			"updated": false,
			"message": "این نصب روی Docker است — برای به‌روزرسانی، از پوشهٔ ریشهٔ پروژه روی همان سرور (همان‌جا که docker-up.ps1 / docker-up.sh هست) دستور زیر را اجرا کنید:\n" +
				"• ویندوز (PowerShell): .\\update.ps1\n" +
				"• لینوکس/مک: ./update.sh\n" +
				"(اگر با اسکریپت نصب لینوکسی docker-install.sh نصب کرده‌اید، دستور «dotinschool update» هم همین کار را می‌کند.)\n" +
				"این اسکریپت‌ها نسخهٔ جدید را از گیت‌هاب می‌گیرند، باینری را دوباره می‌سازند و سرویس را ری‌استارت می‌کنند — بدون نیاز به build داخل کانتینر. داده‌ها در ولوم ./data دست‌نخورده می‌مانند.",
		})
		return
	}

	git := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = s.cfg.Root
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	if _, err := git("fetch", "--depth", "1", "origin", "main"); err != nil {
		fail(c, apperr.New(502, "UPDATE_CHECK_FAILED", "بررسی به‌روزرسانی ناموفق بود — اتصال این سرور به گیت‌هاب را بررسی کنید."))
		return
	}
	localHead, err1 := git("rev-parse", "HEAD")
	remoteHead, err2 := git("rev-parse", "origin/main")
	if err1 != nil || err2 != nil {
		fail(c, apperr.New(502, "UPDATE_CHECK_FAILED", "بررسی به‌روزرسانی ناموفق بود — اتصال این سرور به گیت‌هاب را بررسی کنید."))
		return
	}
	if localHead == remoteHead {
		c.JSON(http.StatusOK, gin.H{"updated": false, "message": "سامانه از قبل به‌روز است — نسخه‌ی جدیدی روی گیت‌هاب نیست."})
		return
	}
	if _, err := git("reset", "--hard", "origin/main"); err != nil {
		fail(c, apperr.New(500, "UPDATE_FAILED", "دریافت نسخه‌ی جدید موفق بود ولی اعمالش شکست خورد — چیزی تغییر نکرد."))
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true, "message": "به‌روزرسانی با موفقیت اعمال شد — سرویس در حال ری‌استارت است."})
	time.AfterFunc(300*time.Millisecond, func() { os.Exit(0) })
}
