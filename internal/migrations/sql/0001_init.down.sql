BEGIN;

DROP INDEX IF EXISTS idx_pr_reviewers_reviewer_id;
DROP INDEX IF EXISTS idx_pull_requests_author_id;
DROP INDEX IF EXISTS idx_team_memberships_user_id;

DROP TABLE IF EXISTS pr_reviewers;
DROP TABLE IF EXISTS pull_requests;
DROP TABLE IF EXISTS pull_request_statuses;
DROP TABLE IF EXISTS team_memberships;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS teams;

COMMIT;
