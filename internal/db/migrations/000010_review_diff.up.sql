-- Add diff storage to the reviews table so substrated no longer needs the
-- agent's repo on its filesystem to produce a diff. The CLI computes the
-- diff locally and ships it in the gRPC request; substrated stores it here
-- and serves it back to the dashboard and reviewer agent from the DB.
ALTER TABLE reviews ADD COLUMN diff_content TEXT;
ALTER TABLE reviews ADD COLUMN diff_command TEXT;
