ALTER TABLE "friendship" DROP CONSTRAINT IF EXISTS "friendship_blocked_by_consistent";
ALTER TABLE "friendship" DROP COLUMN IF EXISTS "blocked_by";
