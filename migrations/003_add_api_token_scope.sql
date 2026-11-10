-- AlterTable: existing tokens stay read-only; "write" must be granted explicitly.
ALTER TABLE "ApiToken" ADD COLUMN "scope" TEXT NOT NULL DEFAULT 'read';
