-- Bound message content. A CHECK is a catalogue-only change, unlike altering
-- the column type which would rewrite the table. NOT VALID enforces the rule
-- on new rows immediately without scanning existing ones; VALIDATE then
-- checks the existing rows under a lock that does not block reads or writes.
ALTER TABLE "message"
  ADD CONSTRAINT "message_content_length"
  CHECK (char_length("content") <= 4000) NOT VALID;

ALTER TABLE "message" VALIDATE CONSTRAINT "message_content_length";

ALTER TABLE "message" ALTER COLUMN "content" SET NOT NULL;
