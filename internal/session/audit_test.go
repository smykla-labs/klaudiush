package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-labs/klaudiush/internal/session"
	"github.com/smykla-labs/klaudiush/pkg/config"
)

var _ = Describe("AuditLogger", func() {
	var (
		logger      *session.AuditLogger
		tempDir     string
		logFile     string
		currentTime time.Time
		timeFunc    func() time.Time
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "session-audit-test-*")
		Expect(err).NotTo(HaveOccurred())

		logFile = filepath.Join(tempDir, "session_audit.jsonl")
		currentTime = time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC)
		timeFunc = func() time.Time { return currentTime }
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("NewAuditLogger", func() {
		It("creates logger with nil config", func() {
			l := session.NewAuditLogger(nil)
			Expect(l).NotTo(BeNil())
		})

		It("creates logger with config", func() {
			l := session.NewAuditLogger(&config.SessionAuditConfig{})
			Expect(l).NotTo(BeNil())
		})

		It("accepts custom log file path", func() {
			l := session.NewAuditLogger(
				nil,
				session.WithAuditFile("/custom/audit.jsonl"),
			)
			Expect(l).NotTo(BeNil())
			Expect(l.GetLogPath()).To(Equal("/custom/audit.jsonl"))
		})

		It("accepts custom time function", func() {
			customTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			l := session.NewAuditLogger(
				nil,
				session.WithAuditTimeFunc(func() time.Time { return customTime }),
			)
			Expect(l).NotTo(BeNil())
		})
	})

	Describe("Log", func() {
		BeforeEach(func() {
			logger = session.NewAuditLogger(
				nil,
				session.WithAuditFile(logFile),
				session.WithAuditTimeFunc(timeFunc),
			)
		})

		It("logs nil entry without error", func() {
			err := logger.Log(nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("logs poison entry", func() {
			entry := &session.AuditEntry{
				Timestamp:     currentTime,
				Action:        session.AuditActionPoison,
				SessionID:     "test-session-123",
				PoisonCodes:   []string{"GIT001", "GIT019"},
				PoisonMessage: "git commit requires -sS flags",
				Command:       "git commit -m \"test\"",
				WorkingDir:    "/project",
			}

			err := logger.Log(entry)
			Expect(err).NotTo(HaveOccurred())

			// Read and verify
			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Action).To(Equal(session.AuditActionPoison))
			Expect(entries[0].SessionID).To(Equal("test-session-123"))
			Expect(entries[0].PoisonCodes).To(Equal([]string{"GIT001", "GIT019"}))
		})

		It("logs unpoison entry", func() {
			entry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionUnpoison,
				SessionID:   "test-session-123",
				PoisonCodes: []string{"GIT001", "GIT019"},
				Source:      "env_var",
				Command:     "KLACK=\"SESS:GIT001,GIT019\" git status",
				WorkingDir:  "/project",
			}

			err := logger.Log(entry)
			Expect(err).NotTo(HaveOccurred())

			// Read and verify
			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Action).To(Equal(session.AuditActionUnpoison))
			Expect(entries[0].Source).To(Equal("env_var"))
		})

		It("logs multiple entries", func() {
			for i := range 5 {
				entry := &session.AuditEntry{
					Timestamp:   currentTime.Add(time.Duration(i) * time.Minute),
					Action:      session.AuditActionPoison,
					SessionID:   "test-session-123",
					PoisonCodes: []string{"GIT001"},
				}
				err := logger.Log(entry)
				Expect(err).NotTo(HaveOccurred())
			}

			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(5))
		})

		It("does not log when disabled", func() {
			enabled := false
			cfg := &config.SessionAuditConfig{
				Enabled: &enabled,
			}
			disabledLogger := session.NewAuditLogger(
				cfg,
				session.WithAuditFile(logFile),
			)

			entry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionPoison,
				SessionID:   "test-session-123",
				PoisonCodes: []string{"GIT001"},
			}

			err := disabledLogger.Log(entry)
			Expect(err).NotTo(HaveOccurred())

			// File should not exist
			_, err = os.Stat(logFile)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})

	Describe("Read", func() {
		BeforeEach(func() {
			logger = session.NewAuditLogger(
				nil,
				session.WithAuditFile(logFile),
				session.WithAuditTimeFunc(timeFunc),
			)
		})

		It("returns empty slice for non-existent file", func() {
			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("skips malformed lines", func() {
			// Write a malformed line
			err := os.WriteFile(logFile, []byte("not json\n"), 0o600)
			Expect(err).NotTo(HaveOccurred())

			// Also write a valid entry
			validEntry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionPoison,
				SessionID:   "test-session",
				PoisonCodes: []string{"GIT001"},
			}
			data, err := json.Marshal(validEntry)
			Expect(err).NotTo(HaveOccurred())

			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o600)
			Expect(err).NotTo(HaveOccurred())
			_, err = f.Write(append(data, '\n'))
			Expect(err).NotTo(HaveOccurred())
			_ = f.Close()

			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].SessionID).To(Equal("test-session"))
		})
	})

	Describe("Rotation", func() {
		BeforeEach(func() {
			cfg := &config.SessionAuditConfig{
				MaxSizeMB: 1, // 1 MB for easier testing
			}
			logger = session.NewAuditLogger(
				cfg,
				session.WithAuditFile(logFile),
				session.WithAuditTimeFunc(timeFunc),
			)
		})

		It("rotates when file exceeds max size", func() {
			// Write a large entry
			largeCommand := make([]byte, 1024*1024) // 1 MB
			for i := range largeCommand {
				largeCommand[i] = 'a'
			}

			entry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionPoison,
				SessionID:   "test-session",
				PoisonCodes: []string{"GIT001"},
				Command:     string(largeCommand),
			}

			err := logger.Log(entry)
			Expect(err).NotTo(HaveOccurred())

			// Write another entry to trigger rotation
			entry2 := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionUnpoison,
				SessionID:   "test-session",
				PoisonCodes: []string{"GIT001"},
			}

			err = logger.Log(entry2)
			Expect(err).NotTo(HaveOccurred())

			// Check for backup file
			files, err := os.ReadDir(tempDir)
			Expect(err).NotTo(HaveOccurred())

			backupFound := false
			for _, f := range files {
				if f.Name() != "session_audit.jsonl" {
					backupFound = true

					break
				}
			}
			Expect(backupFound).To(BeTrue())
		})

		It("rotates manually", func() {
			entry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionPoison,
				SessionID:   "test-session",
				PoisonCodes: []string{"GIT001"},
			}

			err := logger.Log(entry)
			Expect(err).NotTo(HaveOccurred())

			err = logger.Rotate()
			Expect(err).NotTo(HaveOccurred())

			// Check for backup file
			files, err := os.ReadDir(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(files)).To(BeNumerically(">=", 1))
		})
	})

	Describe("Cleanup", func() {
		BeforeEach(func() {
			cfg := &config.SessionAuditConfig{
				MaxAgeDays: 1,
			}
			logger = session.NewAuditLogger(
				cfg,
				session.WithAuditFile(logFile),
				session.WithAuditTimeFunc(timeFunc),
			)
		})

		It("removes old entries", func() {
			// Write an old entry
			oldEntry := &session.AuditEntry{
				Timestamp:   currentTime.Add(-48 * time.Hour), // 2 days ago
				Action:      session.AuditActionPoison,
				SessionID:   "old-session",
				PoisonCodes: []string{"GIT001"},
			}
			data, err := json.Marshal(oldEntry)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(logFile, append(data, '\n'), 0o600)
			Expect(err).NotTo(HaveOccurred())

			// Write a new entry
			newEntry := &session.AuditEntry{
				Timestamp:   currentTime,
				Action:      session.AuditActionUnpoison,
				SessionID:   "new-session",
				PoisonCodes: []string{"GIT001"},
			}
			err = logger.Log(newEntry)
			Expect(err).NotTo(HaveOccurred())

			// Cleanup
			err = logger.Cleanup()
			Expect(err).NotTo(HaveOccurred())

			// Check entries
			entries, err := logger.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].SessionID).To(Equal("new-session"))
		})
	})

	Describe("Stats", func() {
		BeforeEach(func() {
			logger = session.NewAuditLogger(
				nil,
				session.WithAuditFile(logFile),
				session.WithAuditTimeFunc(timeFunc),
			)
		})

		It("returns empty stats for non-existent file", func() {
			stats, err := logger.Stats()
			Expect(err).NotTo(HaveOccurred())
			Expect(stats.EntryCount).To(Equal(0))
			Expect(stats.SizeBytes).To(Equal(int64(0)))
		})

		It("returns accurate stats", func() {
			for range 3 {
				entry := &session.AuditEntry{
					Timestamp:   currentTime,
					Action:      session.AuditActionPoison,
					SessionID:   "test-session",
					PoisonCodes: []string{"GIT001"},
				}
				err := logger.Log(entry)
				Expect(err).NotTo(HaveOccurred())
			}

			stats, err := logger.Stats()
			Expect(err).NotTo(HaveOccurred())
			Expect(stats.EntryCount).To(Equal(3))
			Expect(stats.SizeBytes).To(BeNumerically(">", 0))
		})
	})

	Describe("IsEnabled", func() {
		It("returns true by default", func() {
			l := session.NewAuditLogger(nil)
			Expect(l.IsEnabled()).To(BeTrue())
		})

		It("returns true when explicitly enabled", func() {
			enabled := true
			cfg := &config.SessionAuditConfig{
				Enabled: &enabled,
			}
			l := session.NewAuditLogger(cfg)
			Expect(l.IsEnabled()).To(BeTrue())
		})

		It("returns false when disabled", func() {
			enabled := false
			cfg := &config.SessionAuditConfig{
				Enabled: &enabled,
			}
			l := session.NewAuditLogger(cfg)
			Expect(l.IsEnabled()).To(BeFalse())
		})
	})
})

var _ = Describe("AuditEntry", func() {
	Describe("JSON serialization", func() {
		It("serializes poison entry correctly", func() {
			entry := &session.AuditEntry{
				Timestamp:     time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC),
				Action:        session.AuditActionPoison,
				SessionID:     "test-session-123",
				PoisonCodes:   []string{"GIT001", "GIT019"},
				PoisonMessage: "validation failed",
				Command:       "git commit -m \"test\"",
				WorkingDir:    "/project",
			}

			data, err := json.Marshal(entry)
			Expect(err).NotTo(HaveOccurred())

			var decoded session.AuditEntry
			err = json.Unmarshal(data, &decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded.Action).To(Equal(session.AuditActionPoison))
			Expect(decoded.SessionID).To(Equal("test-session-123"))
			Expect(decoded.PoisonCodes).To(Equal([]string{"GIT001", "GIT019"}))
		})

		It("serializes unpoison entry correctly", func() {
			entry := &session.AuditEntry{
				Timestamp:   time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC),
				Action:      session.AuditActionUnpoison,
				SessionID:   "test-session-123",
				PoisonCodes: []string{"GIT001"},
				Source:      "comment",
				Command:     "git status # SESS:GIT001",
				WorkingDir:  "/project",
			}

			data, err := json.Marshal(entry)
			Expect(err).NotTo(HaveOccurred())

			var decoded session.AuditEntry
			err = json.Unmarshal(data, &decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded.Action).To(Equal(session.AuditActionUnpoison))
			Expect(decoded.Source).To(Equal("comment"))
		})

		It("omits empty fields", func() {
			entry := &session.AuditEntry{
				Timestamp:   time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC),
				Action:      session.AuditActionPoison,
				SessionID:   "test-session",
				PoisonCodes: []string{"GIT001"},
				// Source, PoisonMessage, WorkingDir are empty
			}

			data, err := json.Marshal(entry)
			Expect(err).NotTo(HaveOccurred())

			// Should not contain "source" key
			dataStr := string(data)
			Expect(dataStr).NotTo(ContainSubstring(`"source"`))
			Expect(dataStr).NotTo(ContainSubstring(`"poison_message"`))
		})
	})
})

var _ = Describe("AuditAction", func() {
	Describe("String", func() {
		It("returns correct strings", func() {
			Expect(session.AuditActionPoison.String()).To(Equal("Poison"))
			Expect(session.AuditActionUnpoison.String()).To(Equal("Unpoison"))
		})
	})
})
