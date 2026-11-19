-- blocked_by records who created the block. Wala ni dati, so either side of
-- the pair could set the status back to accepted and lift the other user's
-- block.
ALTER TABLE "friendship" ADD COLUMN "blocked_by" INT REFERENCES "app_user" ("id");

-- Existing blocks predate the column, so wala tay record kung kinsa ang nag-block.
-- Attribute them to user1 so at least one side can still undo it.
UPDATE "friendship" SET "blocked_by" = "user1_id" WHERE "status" = 'blocked';

-- A blocked pair must always remember its blocker, and an accepted pair must
-- never carry one.
ALTER TABLE "friendship"
  ADD CONSTRAINT "friendship_blocked_by_consistent"
  CHECK (
    ("status" = 'blocked' AND "blocked_by" IS NOT NULL) OR
    ("status" <> 'blocked' AND "blocked_by" IS NULL)
  ) NOT VALID;

ALTER TABLE "friendship" VALIDATE CONSTRAINT "friendship_blocked_by_consistent";
