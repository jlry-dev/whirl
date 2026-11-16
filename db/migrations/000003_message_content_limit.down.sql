ALTER TABLE "message" ALTER COLUMN "content" DROP NOT NULL;
ALTER TABLE "message" DROP CONSTRAINT IF EXISTS "message_content_length";
