-- Indexes for the two hot read paths.

-- Message history is fetched by participant pair, newest first. Equality
-- columns come first so the pair is pinned, then the sort columns match the
-- ORDER BY exactly (including the id tiebreaker) so no extra sort is needed
-- within a conversation. One index serves both OR branches of the query.
CREATE INDEX IF NOT EXISTS idx_message_pair_ts
  ON "message" ("sender_id", "receiver_id", "timestamp" DESC, "id" DESC);

-- Friendships are stored in canonical order (user1_id < user2_id), so a given
-- user can appear in either column. The primary key covers user1_id only;
-- this covers lookups from the other direction. The trailing columns allow an
-- index-only scan and cover the status filter.
CREATE INDEX IF NOT EXISTS idx_friendship_user2
  ON "friendship" ("user2_id", "user1_id", "status");
