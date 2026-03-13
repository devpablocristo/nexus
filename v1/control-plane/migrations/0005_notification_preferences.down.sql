DROP INDEX IF EXISTS idx_notification_log_dedup_key;
DROP INDEX IF EXISTS idx_notification_log_org_created;
DROP TABLE IF EXISTS notification_log;

DROP INDEX IF EXISTS idx_notification_prefs_user;
DROP TABLE IF EXISTS notification_preferences;
